package opensearch

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/aiven"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"github.com/nais/pgrator/pkg/api"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var (
	specDiskSpace             = []string{"spec", "disk_space"}
	specTerminationProtection = []string{"spec", "terminationProtection"}
	specOpenSearchVersion     = []string{"spec", "userConfig", "opensearch_version"}
)

func GetByIdent(ctx context.Context, id ident.Ident) (*OpenSearch, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*OpenSearch, error) {
	v, err := fromContext(ctx).naisWatcher.Get(environment, teamSlug.String(), name)
	if errors.Is(err, &watcher.ErrorNotFound{}) {
		prefix := instanceNamer(teamSlug, "")
		if !strings.HasPrefix(name, prefix) {
			name = instanceNamer(teamSlug, name)
		}
		v, err = fromContext(ctx).watcher.Get(environment, teamSlug.String(), name)
	}
	return v, err
}

func State(ctx context.Context, os *OpenSearch) (OpenSearchState, error) {
	project, err := aiven.GetProject(ctx, os.EnvironmentName)
	if err != nil {
		return OpenSearchStateUnknown, err
	}
	s, err := fromContext(ctx).aivenClient.ServiceGet(ctx, project.ID, os.FullyQualifiedName())
	if err != nil {
		// The OpenSearch instance may not have been created in Aiven yet, or it has been deleted.
		// In both cases, we return "unknown" state rather than an error.
		if aiven.IsNotFound(err) {
			return OpenSearchStateUnknown, nil
		}
		return OpenSearchStateUnknown, err
	}

	switch s.State {
	case "RUNNING":
		return OpenSearchStateRunning, nil
	case "REBALANCING":
		return OpenSearchStateRebalancing, nil
	case "REBUILDING":
		return OpenSearchStateRebuilding, nil
	case "POWEROFF":
		return OpenSearchStatePoweroff, nil
	default:
		return OpenSearchStateUnknown, nil
	}
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *OpenSearchOrder) (*OpenSearchConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderOpenSearch(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*OpenSearch {
	all := fromContext(ctx).watcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted())
	allNais := fromContext(ctx).naisWatcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted())
	all = append(all, allNais...)
	return watcher.Objects(all)
}

func ListAccess(ctx context.Context, openSearch *OpenSearch, page *pagination.Pagination, orderBy *OpenSearchAccessOrder) (*OpenSearchAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, openSearch.EnvironmentName, openSearch.FullyQualifiedName(), openSearch.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, openSearch.EnvironmentName, openSearch.FullyQualifiedName(), openSearch.TeamSlug)
	if err != nil {
		return nil, err
	}

	all := make([]*OpenSearchAccess, 0)
	all = append(all, applicationAccess...)
	all = append(all, jobAccess...)

	if orderBy == nil {
		orderBy = &OpenSearchAccessOrder{
			Field:     "ACCESS",
			Direction: model.OrderDirectionAsc,
		}
	}
	SortFilterOpenSearchAccess.Sort(ctx, all, orderBy.Field, orderBy.Direction)

	ret := pagination.Slice(all, page)
	return pagination.NewConnection(ret, page, len(all)), nil
}

func GetOpenSearchVersion(ctx context.Context, os *OpenSearch) (*OpenSearchVersion, error) {
	project, err := aiven.GetProject(ctx, os.EnvironmentName)
	if err != nil {
		return nil, err
	}
	key := AivenDataLoaderKey{
		Project:     project.ID,
		ServiceName: os.FullyQualifiedName(),
	}

	major := os.MajorVersion
	var versionString *string
	v, err := fromContext(ctx).versionLoader.Load(ctx, &key)
	if err == nil {
		versionString = ptr.To(v)
		if major == "" {
			mv, err := OpenSearchMajorVersionFromAivenString(v)
			if err != nil {
				return nil, err
			}
			major = mv
		}
	}

	if major == "" {
		major = OpenSearchMajorVersionV2
	}

	return &OpenSearchVersion{
		DesiredMajor: major,
		Actual:       versionString,
	}, nil
}

func GetForWorkload(ctx context.Context, teamSlug slug.Slug, environment string, reference *nais_io_v1.OpenSearch) (*OpenSearch, error) {
	if reference == nil {
		return nil, nil
	}

	return Get(ctx, teamSlug, environment, reference.Instance)
}

func orderOpenSearch(ctx context.Context, ret []*OpenSearch, orderBy *OpenSearchOrder) {
	if orderBy == nil {
		orderBy = &OpenSearchOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilterOpenSearch.Sort(ctx, ret, orderBy.Field, orderBy.Direction)
}

func Create(ctx context.Context, input CreateOpenSearchInput) (*CreateOpenSearchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := newK8sClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	// Ensure there's no existing Aiven OpenSearch with the same name
	// This can be removed when we manage all opensearches through Console
	_, err = fromContext(ctx).watcher.Get(input.EnvironmentName, input.TeamSlug.String(), instanceNamer(input.TeamSlug, input.Name))
	if !errors.Is(err, &watcher.ErrorNotFound{}) {
		return nil, apierror.Errorf("OpenSearch with the name %q already exists, but are not yet managed through Console.", input.Name)
	}

	res := &naiscrd.OpenSearch{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OpenSearch",
			APIVersion: "nais.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.TeamSlug.String(),
		},
		Spec: naiscrd.OpenSearchSpec{
			Tier:      toMapperatorTier(input.Tier),
			Memory:    toMapperatorMemory(input.Memory),
			Version:   toMapperatorVersion(input.Version),
			StorageGB: int(input.StorageGB),
		},
	}
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	obj, err := kubernetes.ToUnstructured(res)
	if err != nil {
		return nil, err
	}

	if _, err = client.Create(ctx, obj, metav1.CreateOptions{}); err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionCreated,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeOpenSearch,
		ResourceName:    input.Name,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
	})
	if err != nil {
		return nil, err
	}

	os, err := toOpenSearchFromNais(res, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &CreateOpenSearchPayload{
		OpenSearch: os,
	}, nil
}

func Update(ctx context.Context, input UpdateOpenSearchInput) (*UpdateOpenSearchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := newK8sClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	openSearch, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	concreteOpenSearch, err := kubernetes.ToConcrete[naiscrd.OpenSearch](openSearch)
	if err != nil {
		return nil, err
	}

	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)
	updateFuncs := []func(*naiscrd.OpenSearch, UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error){
		updateTier,
		updateMemory,
		updateVersion,
		updateStorage,
	}

	for _, f := range updateFuncs {
		res, err := f(concreteOpenSearch, input)
		if err != nil {
			return nil, err
		}
		changes = append(changes, res...)
	}

	if len(changes) == 0 {
		os, err := toOpenSearch(openSearch, input.EnvironmentName)
		if err != nil {
			return nil, err
		}

		return &UpdateOpenSearchPayload{
			OpenSearch: os,
		}, nil
	}

	obj, err := kubernetes.ToUnstructured(concreteOpenSearch)
	if err != nil {
		return nil, err
	}

	obj.SetAnnotations(kubernetes.WithCommonAnnotations(obj.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))

	ret, err := client.Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeOpenSearch,
		ResourceName:    input.Name,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
		Data: &OpenSearchUpdatedActivityLogEntryData{
			UpdatedFields: changes,
		},
	})
	if err != nil {
		return nil, err
	}

	retOpenSearch, err := kubernetes.ToConcrete[naiscrd.OpenSearch](ret)
	if err != nil {
		return nil, err
	}

	osUpdated, err := toOpenSearchFromNais(retOpenSearch, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &UpdateOpenSearchPayload{
		OpenSearch: osUpdated,
	}, nil
}

func updateTier(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)

	origTier := fromMapperatorTier(openSearch.Spec.Tier)
	if input.Tier != origTier {
		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field:    "tier",
			OldValue: ptr.To(origTier.String()),
			NewValue: ptr.To(input.Tier.String()),
		})
	}

	openSearch.Spec.Tier = toMapperatorTier(input.Tier)

	return changes, nil
}

func updateMemory(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)

	origMemory := fromMapperatorMemory(openSearch.Spec.Memory)
	if input.Memory != origMemory {
		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field:    "memory",
			OldValue: ptr.To(origMemory.String()),
			NewValue: ptr.To(input.Memory.String()),
		})
	}

	openSearch.Spec.Memory = toMapperatorMemory(input.Memory)

	return changes, nil
}

func updateVersion(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	origVersion := fromMapperatorVersion(openSearch.Spec.Version)

	if origVersion == input.Version {
		return nil, nil
	}

	if err := input.Version.ValidateUpgradePath(origVersion); err != nil {
		return nil, err
	}

	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)

	var oldValue *string
	if origVersion != "" {
		oldValue = ptr.To(origVersion.String())
	}

	changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
		Field:    "version",
		OldValue: oldValue,
		NewValue: ptr.To(input.Version.String()),
	})

	openSearch.Spec.Version = toMapperatorVersion(input.Version)

	return changes, nil
}

func updateStorage(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	oldStorageGB := StorageGB(openSearch.Spec.StorageGB)

	if oldStorageGB == input.StorageGB {
		return nil, nil
	}

	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)

	var oldValue *string
	if oldStorageGB > 0 {
		oldValue = ptr.To(oldStorageGB.String())
	}

	changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
		Field:    "storageGB",
		OldValue: oldValue,
		NewValue: ptr.To(input.StorageGB.String()),
	})

	openSearch.Spec.StorageGB = int(input.StorageGB)

	return changes, nil
}

func Delete(ctx context.Context, input DeleteOpenSearchInput) (*DeleteOpenSearchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	client, err := newK8sClient(ctx, input.EnvironmentName, input.TeamSlug)
	if err != nil {
		return nil, err
	}

	openSearch, err := client.Get(ctx, input.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !kubernetes.HasManagedByConsoleLabel(openSearch) {
		return nil, apierror.Errorf("OpenSearch %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	annotations := openSearch.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if annotations[api.AllowDeletionAnnotation] != "true" {
		annotations[api.AllowDeletionAnnotation] = "true"
		openSearch.SetAnnotations(annotations)

		_, err = client.Update(ctx, openSearch, metav1.UpdateOptions{})
		if err != nil {
			return nil, fmt.Errorf("set allow deletion annotation: %w", err)
		}
	}

	if err := client.Delete(ctx, input.Name, metav1.DeleteOptions{}); err != nil {
		return nil, err
	}

	if err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypeOpenSearch,
		ResourceName:    input.Name,
		EnvironmentName: ptr.To(input.EnvironmentName),
		TeamSlug:        ptr.To(input.TeamSlug),
	}); err != nil {
		return nil, err
	}

	return &DeleteOpenSearchPayload{
		OpenSearchDeleted: ptr.To(true),
	}, nil
}

func toMapperatorTier(tier OpenSearchTier) naiscrd.OpenSearchTier {
	switch tier {
	case OpenSearchTierSingleNode:
		return naiscrd.OpenSearchTierSingleNode
	case OpenSearchTierHighAvailability:
		return naiscrd.OpenSearchTierHighAvailability
	default:
		return ""
	}
}

func fromMapperatorTier(tier naiscrd.OpenSearchTier) OpenSearchTier {
	switch tier {
	case naiscrd.OpenSearchTierSingleNode:
		return OpenSearchTierSingleNode
	case naiscrd.OpenSearchTierHighAvailability:
		return OpenSearchTierHighAvailability
	default:
		return ""
	}
}

func toMapperatorMemory(memory OpenSearchMemory) naiscrd.OpenSearchMemory {
	switch memory {
	case OpenSearchMemoryGB2:
		return naiscrd.OpenSearchMemory2GB
	case OpenSearchMemoryGB4:
		return naiscrd.OpenSearchMemory4GB
	case OpenSearchMemoryGB8:
		return naiscrd.OpenSearchMemory8GB
	case OpenSearchMemoryGB16:
		return naiscrd.OpenSearchMemory16GB
	case OpenSearchMemoryGB32:
		return naiscrd.OpenSearchMemory32GB
	case OpenSearchMemoryGB64:
		return naiscrd.OpenSearchMemory64GB
	default:
		return ""
	}
}

func fromMapperatorMemory(memory naiscrd.OpenSearchMemory) OpenSearchMemory {
	switch memory {
	case naiscrd.OpenSearchMemory2GB:
		return OpenSearchMemoryGB2
	case naiscrd.OpenSearchMemory4GB:
		return OpenSearchMemoryGB4
	case naiscrd.OpenSearchMemory8GB:
		return OpenSearchMemoryGB8
	case naiscrd.OpenSearchMemory16GB:
		return OpenSearchMemoryGB16
	case naiscrd.OpenSearchMemory32GB:
		return OpenSearchMemoryGB32
	case naiscrd.OpenSearchMemory64GB:
		return OpenSearchMemoryGB64
	default:
		return ""
	}
}

func toMapperatorVersion(version OpenSearchMajorVersion) naiscrd.OpenSearchVersion {
	switch version {
	case OpenSearchMajorVersionV1:
		return naiscrd.OpenSearchVersionV1
	case OpenSearchMajorVersionV2:
		return naiscrd.OpenSearchVersionV2
	case OpenSearchMajorVersionV2_19:
		return naiscrd.OpenSearchVersionV2_19
	case OpenSearchMajorVersionV3_3:
		return naiscrd.OpenSearchVersionV3_3
	default:
		return ""
	}
}

func fromMapperatorVersion(version naiscrd.OpenSearchVersion) OpenSearchMajorVersion {
	switch version {
	case naiscrd.OpenSearchVersionV1:
		return OpenSearchMajorVersionV1
	case naiscrd.OpenSearchVersionV2:
		return OpenSearchMajorVersionV2
	case naiscrd.OpenSearchVersionV2_19:
		return OpenSearchMajorVersionV2_19
	case naiscrd.OpenSearchVersionV3_3:
		return OpenSearchMajorVersionV3_3
	default:
		return ""
	}
}
