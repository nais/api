package opensearch

import (
	"context"
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
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

func GetByIdent(ctx context.Context, id ident.Ident) (*OpenSearch, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*OpenSearch, error) {
	prefix := instanceNamer(teamSlug, "")
	if !strings.HasPrefix(name, prefix) {
		name = instanceNamer(teamSlug, name)
	}
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func State(ctx context.Context, os *OpenSearch) (OpenSearchState, error) {
	s, err := fromContext(ctx).aivenClient.ServiceGet(ctx, os.AivenProject, os.FullyQualifiedName())
	if err != nil {
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
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String(), watcher.WithoutDeleted())
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
	key := AivenDataLoaderKey{
		Project:     os.AivenProject,
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

	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), instanceNamer(teamSlug, reference.Instance))
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

	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	machine, err := machineTypeFromTierAndMemory(input.Tier, input.Memory)
	if err != nil {
		return nil, err
	}

	name := instanceNamer(input.TeamSlug, input.Name)
	namespace := input.TeamSlug.String()

	res := &unstructured.Unstructured{}
	res.SetAPIVersion("aiven.io/v1alpha1")
	res.SetKind("OpenSearch")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	aivenProject, err := aiven.GetProject(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	res.Object["spec"] = map[string]any{
		"cloudName":             "google-europe-north1",
		"plan":                  machine.AivenPlan,
		"project":               aivenProject.ID,
		"projectVpcId":          aivenProject.VPC,
		"disk_space":            input.StorageGB.ToAivenString(),
		"terminationProtection": true,
		"tags": map[string]any{
			"environment": input.EnvironmentName,
			"team":        namespace,
			"tenant":      fromContext(ctx).tenantName,
		},
		"userConfig": map[string]any{
			"opensearch_version": input.Version.ToAivenString(),
		},
	}

	ret, err := client.Namespace(namespace).Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
		return nil, err
	}

	err = aiven.UpsertPrometheusServiceIntegration(ctx, fromContext(ctx).watcher, ret, aivenProject, input.EnvironmentName)
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus service integration: %w", err)
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

	os, err := toOpenSearch(ret, input.EnvironmentName)
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

	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	name := instanceNamer(input.TeamSlug, input.Name)
	namespace := input.TeamSlug.String()

	openSearch, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !kubernetes.HasManagedByConsoleLabel(openSearch) {
		return nil, apierror.Errorf("OpenSearch %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	changes := []*OpenSearchUpdatedActivityLogEntryDataUpdatedField{}

	machine, err := machineTypeFromTierAndMemory(input.Tier, input.Memory)
	if err != nil {
		return nil, err
	}

	oldPlan, found, err := unstructured.NestedString(openSearch.Object, "spec", "plan")
	if err != nil || !found {
		return nil, err
	}

	if oldPlan != machine.AivenPlan {
		err = unstructured.SetNestedField(openSearch.Object, machine.AivenPlan, "spec", "plan")
		if err != nil {
			return nil, err
		}

		oldMachine, err := machineTypeFromPlan(oldPlan)
		if err != nil {
			return nil, err
		}

		if input.Tier != oldMachine.Tier {
			changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
				Field: "tier",
				OldValue: func() *string {
					if found {
						return ptr.To(oldMachine.Tier.String())
					}
					return nil
				}(),
				NewValue: ptr.To(input.Tier.String()),
			})
		}

		if input.Memory != oldMachine.Memory {
			changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
				Field: "memory",
				OldValue: func() *string {
					if found {
						return ptr.To(oldMachine.Memory.String())
					}
					return nil
				}(),
				NewValue: ptr.To(input.Memory.String()),
			})
		}
	}

	oldVersion, found, err := unstructured.NestedString(openSearch.Object, "spec", "userConfig", "opensearch_version")
	if err != nil {
		return nil, err
	}
	if !found {
		os, err := toOpenSearch(openSearch, input.EnvironmentName)
		if err != nil {
			return nil, err
		}
		version, err := GetOpenSearchVersion(ctx, os)
		if err != nil {
			return nil, err
		}

		oldVersion = *version.Actual
	}
	oldMajorVersion, err := OpenSearchMajorVersionFromAivenString(oldVersion)
	if err != nil {
		return nil, err
	}
	if oldMajorVersion != input.Version {
		if input.Version.IsDowngradeTo(oldMajorVersion) {
			return nil, apierror.Errorf("Cannot downgrade OpenSearch version from %v to %v", oldMajorVersion, input.Version.String())
		}

		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field: "version",
			OldValue: func() *string {
				if found {
					return ptr.To(oldVersion)
				}
				return nil
			}(),
			NewValue: ptr.To(input.Version.String()),
		})
		err = unstructured.SetNestedField(openSearch.Object, input.Version.ToAivenString(), "spec", "userConfig", "opensearch_version")
		if err != nil {
			return nil, err
		}
	}

	oldAivenDiskSpace, found, err := unstructured.NestedString(openSearch.Object, "spec", "disk_space")
	if err != nil {
		return nil, err
	}
	// default to minimum storage capacity for the selected plan, in case the field is not set explicitly
	oldStorageGB := machine.StorageMin
	if found {
		oldStorageGB, err = StorageGBFromAivenString(oldAivenDiskSpace)
		if err != nil {
			return nil, err
		}
	}
	if oldStorageGB != input.StorageGB {
		changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
			Field: "storageGB",
			OldValue: func() *string {
				if found {
					return ptr.To(oldStorageGB.String())
				}
				return nil
			}(),
			NewValue: ptr.To(input.StorageGB.String()),
		})
		err = unstructured.SetNestedField(openSearch.Object, input.StorageGB.ToAivenString(), "spec", "disk_space")
		if err != nil {
			return nil, err
		}
	}

	if len(changes) == 0 {
		// No changes to update
		os, err := toOpenSearch(openSearch, input.EnvironmentName)
		if err != nil {
			return nil, err
		}

		return &UpdateOpenSearchPayload{
			OpenSearch: os,
		}, nil
	}

	openSearch.SetAnnotations(kubernetes.WithCommonAnnotations(openSearch.GetAnnotations(), authz.ActorFromContext(ctx).User.Identity()))

	ret, err := client.Namespace(namespace).Update(ctx, openSearch, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	aivenProject, err := aiven.GetProject(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	err = aiven.UpsertPrometheusServiceIntegration(ctx, fromContext(ctx).watcher, ret, aivenProject, input.EnvironmentName)
	if err != nil {
		return nil, fmt.Errorf("creating Prometheus service integration: %w", err)
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

	os, err := toOpenSearch(ret, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	return &UpdateOpenSearchPayload{
		OpenSearch: os,
	}, nil
}

func Delete(ctx context.Context, input DeleteOpenSearchInput) (*DeleteOpenSearchPayload, error) {
	if err := input.Validate(ctx); err != nil {
		return nil, err
	}

	name := instanceNamer(input.TeamSlug, input.Name)
	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	nsclient := client.Namespace(input.TeamSlug.String())

	os, err := nsclient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if !kubernetes.HasManagedByConsoleLabel(os) {
		return nil, apierror.Errorf("OpenSearch %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	terminationProtection, found, err := unstructured.NestedBool(os.Object, "spec", "terminationProtection")
	if err != nil {
		return nil, err
	}
	if found && terminationProtection {
		if err := unstructured.SetNestedField(os.Object, false, "spec", "terminationProtection"); err != nil {
			return nil, err
		}

		if _, err = nsclient.Update(ctx, os, metav1.UpdateOptions{}); err != nil {
			return nil, fmt.Errorf("removing deletion protection: %w", err)
		}
	}

	if err := fromContext(ctx).watcher.Delete(ctx, input.EnvironmentName, input.TeamSlug.String(), name); err != nil {
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
