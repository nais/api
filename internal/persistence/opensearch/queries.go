package opensearch

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/aivencredentials"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/aiven"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"github.com/nais/pgrator/pkg/api"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	specDiskSpace             = []string{"spec", "disk_space"}
	specTerminationProtection = []string{"spec", "terminationProtection"}
	specOpenSearchVersion     = []string{"spec", "userConfig", "opensearch_version"}

	specShardIndexingPressureEnabled  = []string{"spec", "userConfig", "opensearch", "shard_indexing_pressure", "enabled"}
	specShardIndexingPressureEnforced = []string{"spec", "userConfig", "opensearch", "shard_indexing_pressure", "enforced"}

	specIndicesQueryBoolMaxClauseCount = []string{"spec", "userConfig", "opensearch", "indices_query_bool_max_clause_count"}

	specHTTPMaxContentLength = []string{"spec", "userConfig", "opensearch", "http_max_content_length"}
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

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *OpenSearchOrder, filter *OpenSearchFilter) (*OpenSearchConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)

	if orderBy == nil {
		orderBy = &OpenSearchOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	return SortFilterOpenSearch.PaginatedList(ctx, all, page, orderBy.Field, orderBy.Direction, filter), nil
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
		versionString = new(v)
		if major == "" {
			mv, err := OpenSearchMajorVersionFromAivenString(v)
			if err != nil {
				return nil, err
			}
			major = mv
		}
	}

	if major == "" {
		major = OpenSearchMajorVersionV3_3
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

	if input.ShardIndexingPressure != nil {
		res.Spec.ShardIndexingPressure = &naiscrd.OpenSearchShardIndexingPressure{
			Enabled:  input.ShardIndexingPressure.Enabled,
			Enforced: input.ShardIndexingPressure.Enforced,
		}
	}

	if input.Indices != nil && input.Indices.QueryBoolMaxClauseCount != nil {
		res.Spec.Indices = &naiscrd.OpenSearchIndices{
			QueryBoolMaxClauseCount: input.Indices.QueryBoolMaxClauseCount,
		}
	}

	if input.HTTP != nil && input.HTTP.MaxContentLength != nil {
		q, err := resource.ParseQuantity(*input.HTTP.MaxContentLength)
		if err != nil {
			return nil, err
		}
		res.Spec.Http = &naiscrd.OpenSearchHttp{
			MaxContentLength: &q,
		}
	}

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
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
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
		updateShardIndexingPressure,
		updateIndices,
		updateHTTP,
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
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
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

func CreateOpenSearchCredentials(ctx context.Context, input CreateOpenSearchCredentialsInput) (*CreateOpenSearchCredentialsPayload, error) {
	// Strip "opensearch-<team>-" prefix if the user provided the full Kubernetes resource name.
	// The buildSpec already prepends "opensearch-<namespace>-" for the Aivenator.
	instanceName := strings.TrimPrefix(input.InstanceName, NamePrefix(input.TeamSlug))
	req := aivencredentials.CredentialRequest{
		TeamSlug:        input.TeamSlug,
		EnvironmentName: input.EnvironmentName,
		TTL:             input.TTL,
		InstanceName:    instanceName,
		Permission:      input.Permission.String(),
		MaxTTL:          aivencredentials.MaxTTLDefault,
		BuildSpec: func(namespace, secretName string, expiresAt time.Time) map[string]any {
			return map[string]any{
				"protected": true,
				"expiresAt": expiresAt.Format(time.RFC3339),
				"openSearch": map[string]any{
					"instance":   fmt.Sprintf("opensearch-%s-%s", namespace, instanceName),
					"access":     input.Permission.AivenAccess(),
					"secretName": secretName,
				},
			}
		},
		ExtractCreds: func(data map[string]string) any {
			port, _ := strconv.Atoi(data["OPEN_SEARCH_PORT"])
			return &OpenSearchCredentials{
				Username: data["OPEN_SEARCH_USERNAME"],
				Password: data["OPEN_SEARCH_PASSWORD"],
				Host:     data["OPEN_SEARCH_HOST"],
				Port:     port,
				URI:      data["OPEN_SEARCH_URI"],
			}
		},
	}
	result, err := aivencredentials.CreateCredentials(ctx, ActivityLogEntryResourceTypeOpenSearch, req)
	if err != nil {
		return nil, err
	}
	aivencredentials.LogCredentialCreation(ctx, ActivityLogEntryResourceTypeOpenSearch, req)
	return &CreateOpenSearchCredentialsPayload{Credentials: result.(*OpenSearchCredentials)}, nil
}

func updateTier(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)

	origTier := fromMapperatorTier(openSearch.Spec.Tier)
	if input.Tier != origTier {
		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field:    "tier",
			OldValue: new(origTier.String()),
			NewValue: new(input.Tier.String()),
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
			OldValue: new(origMemory.String()),
			NewValue: new(input.Memory.String()),
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
		oldValue = new(origVersion.String())
	}

	changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
		Field:    "version",
		OldValue: oldValue,
		NewValue: new(input.Version.String()),
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
		oldValue = new(oldStorageGB.String())
	}

	changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
		Field:    "storageGB",
		OldValue: oldValue,
		NewValue: new(input.StorageGB.String()),
	})

	openSearch.Spec.StorageGB = int(input.StorageGB)

	return changes, nil
}

func updateShardIndexingPressure(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	if input.ShardIndexingPressure == nil {
		return nil, nil
	}

	var oldEnabled, oldEnforced bool
	if openSearch.Spec.ShardIndexingPressure != nil {
		oldEnabled = openSearch.Spec.ShardIndexingPressure.Enabled
		oldEnforced = openSearch.Spec.ShardIndexingPressure.Enforced
	}

	changes := make([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, 0)

	if oldEnabled != input.ShardIndexingPressure.Enabled {
		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field:    "shardIndexingPressure.enabled",
			OldValue: new(strconv.FormatBool(oldEnabled)),
			NewValue: new(strconv.FormatBool(input.ShardIndexingPressure.Enabled)),
		})
	}
	if oldEnforced != input.ShardIndexingPressure.Enforced {
		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field:    "shardIndexingPressure.enforced",
			OldValue: new(strconv.FormatBool(oldEnforced)),
			NewValue: new(strconv.FormatBool(input.ShardIndexingPressure.Enforced)),
		})
	}

	if len(changes) == 0 {
		return nil, nil
	}

	openSearch.Spec.ShardIndexingPressure = &naiscrd.OpenSearchShardIndexingPressure{
		Enabled:  input.ShardIndexingPressure.Enabled,
		Enforced: input.ShardIndexingPressure.Enforced,
	}

	return changes, nil
}

func updateIndices(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	if input.Indices == nil {
		return nil, nil
	}

	var oldCount *int
	if openSearch.Spec.Indices != nil {
		oldCount = openSearch.Spec.Indices.QueryBoolMaxClauseCount
	}
	newCount := input.Indices.QueryBoolMaxClauseCount

	if equalIntPtr(oldCount, newCount) {
		return nil, nil
	}

	openSearch.Spec.Indices = &naiscrd.OpenSearchIndices{
		QueryBoolMaxClauseCount: newCount,
	}

	return []*OpenSearchUpdatedActivityLogEntryDataUpdatedField{
		{
			Field:    "indices.queryBoolMaxClauseCount",
			OldValue: intPtrToStringPtr(oldCount),
			NewValue: intPtrToStringPtr(newCount),
		},
	}, nil
}

func updateHTTP(openSearch *naiscrd.OpenSearch, input UpdateOpenSearchInput) ([]*OpenSearchUpdatedActivityLogEntryDataUpdatedField, error) {
	if input.HTTP == nil || input.HTTP.MaxContentLength == nil {
		return nil, nil
	}

	newQ, err := resource.ParseQuantity(*input.HTTP.MaxContentLength)
	if err != nil {
		return nil, err
	}

	var oldValue *string
	if openSearch.Spec.Http != nil && openSearch.Spec.Http.MaxContentLength != nil {
		old := openSearch.Spec.Http.MaxContentLength
		if old.Cmp(newQ) == 0 {
			return nil, nil
		}
		s := old.String()
		oldValue = &s
	}

	openSearch.Spec.Http = &naiscrd.OpenSearchHttp{
		MaxContentLength: &newQ,
	}

	return []*OpenSearchUpdatedActivityLogEntryDataUpdatedField{
		{
			Field:    "http.maxContentLength",
			OldValue: oldValue,
			NewValue: new(newQ.String()),
		},
	}, nil
}

func equalIntPtr(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func intPtrToStringPtr(v *int) *string {
	if v == nil {
		return nil
	}
	return new(strconv.Itoa(*v))
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
		EnvironmentName: new(input.EnvironmentName),
		TeamSlug:        new(input.TeamSlug),
	}); err != nil {
		return nil, err
	}

	return &DeleteOpenSearchPayload{
		OpenSearchDeleted: new(true),
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
