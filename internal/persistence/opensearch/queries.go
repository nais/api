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
	prefix := openSearchNamer(teamSlug, "")
	if !strings.HasPrefix(name, prefix) {
		name = openSearchNamer(teamSlug, name)
	}
	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), name)
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *OpenSearchOrder) (*OpenSearchConnection, error) {
	all := ListAllForTeam(ctx, teamSlug)
	orderOpenSearch(ctx, all, orderBy)

	instances := pagination.Slice(all, page)
	return pagination.NewConnection(instances, page, len(all)), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*OpenSearch {
	all := fromContext(ctx).client.watcher.GetByNamespace(teamSlug.String())
	return watcher.Objects(all)
}

func ListAccess(ctx context.Context, openSearch *OpenSearch, page *pagination.Pagination, orderBy *OpenSearchAccessOrder) (*OpenSearchAccessConnection, error) {
	k8sClient := fromContext(ctx).client

	applicationAccess, err := k8sClient.getAccessForApplications(ctx, openSearch.EnvironmentName, openSearch.Name, openSearch.TeamSlug)
	if err != nil {
		return nil, err
	}

	jobAccess, err := k8sClient.getAccessForJobs(ctx, openSearch.EnvironmentName, openSearch.Name, openSearch.TeamSlug)
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
		ServiceName: os.Name,
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

	return fromContext(ctx).client.watcher.Get(environment, teamSlug.String(), openSearchNamer(teamSlug, reference.Instance))
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

	plan, err := planFromTierAndSize(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}

	name := openSearchNamer(input.TeamSlug, input.Name)
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
	version := strings.TrimLeft(input.Version.String(), "V")

	res.Object["spec"] = map[string]any{
		"cloudName":             "google-europe-north1",
		"plan":                  plan,
		"project":               aivenProject.ID,
		"projectVpcId":          aivenProject.VPC,
		"terminationProtection": true,
		"tags": map[string]any{
			"environment": input.EnvironmentName,
			"team":        namespace,
			"tenant":      fromContext(ctx).tenantName,
		},
		"userConfig": map[string]any{
			"opensearch_version": version,
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

	name := openSearchNamer(input.TeamSlug, input.Name)
	namespace := input.TeamSlug.String()

	openSearch, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !kubernetes.HasManagedByConsoleLabel(openSearch) {
		return nil, apierror.Errorf("OpenSearch %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	changes := []*OpenSearchUpdatedActivityLogEntryDataUpdatedField{}

	plan, err := planFromTierAndSize(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}

	oldPlan, found, err := unstructured.NestedString(openSearch.Object, "spec", "plan")
	if err != nil {
		return nil, err
	}

	if !found || oldPlan != plan {
		err = unstructured.SetNestedField(openSearch.Object, plan, "spec", "plan")
		if err != nil {
			return nil, err
		}

		tier, size, err := tierAndSizeFromPlan(oldPlan)
		if err != nil {
			return nil, err
		}

		if input.Tier != tier {
			changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
				Field: "tier",
				OldValue: func() *string {
					if found {
						return ptr.To(tier.String())
					}
					return nil
				}(),
				NewValue: ptr.To(input.Tier.String()),
			})
		}
		if input.Size != size {
			changes = append(changes, &OpenSearchUpdatedActivityLogEntryDataUpdatedField{
				Field: "size",
				OldValue: func() *string {
					if found {
						return ptr.To(size.String())
					}
					return nil
				}(),
				NewValue: ptr.To(input.Size.String()),
			})
		}
	}

	oldVersion, found, err := unstructured.NestedString(openSearch.Object, "spec", "userConfig", "opensearch_version")
	if err != nil {
		return nil, err
	}
	if !found || oldVersion != input.Version.String() {
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

	name := openSearchNamer(input.TeamSlug, input.Name)
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

	if err := nsclient.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
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

var aivenPlans = map[string]OpenSearchTier{
	"business": OpenSearchTierHighAvailability,
	"startup":  OpenSearchTierSingleNode,
}

var aivenSizes = map[string]OpenSearchSize{
	"2":  OpenSearchSizeRAM2gb,
	"4":  OpenSearchSizeRAM4gb,
	"8":  OpenSearchSizeRAM8gb,
	"16": OpenSearchSizeRAM16gb,
	"32": OpenSearchSizeRAM32gb,
	"64": OpenSearchSizeRAM64gb,
}

func planFromTierAndSize(tier OpenSearchTier, size OpenSearchSize) (string, error) {
	if tier == OpenSearchTierSingleNode && size == OpenSearchSizeRAM2gb {
		return "hobbyist", nil
	}
	if tier == OpenSearchTierHighAvailability && size == OpenSearchSizeRAM2gb {
		return "", apierror.Errorf("Invalid OpenSearch size for tier. %v cannot have size %v", tier, size)
	}

	plan := ""

	for name, planTier := range aivenPlans {
		if planTier == tier {
			plan = name + "-"
			break
		}
	}
	if plan == "" {
		return "", apierror.Errorf("invalid OpenSearch tier: %s", tier)
	}

	planSize := ""
	for name, sz := range aivenSizes {
		if sz == size {
			planSize = name
			break
		}
	}
	if planSize == "" {
		return "", apierror.Errorf("invalid OpenSearch size: %s", size)
	}
	plan += planSize

	return plan, nil
}

func tierAndSizeFromPlan(plan string) (OpenSearchTier, OpenSearchSize, error) {
	if strings.EqualFold(plan, "hobbyist") {
		return OpenSearchTierSingleNode, OpenSearchSizeRAM2gb, nil
	}

	t, s, ok := strings.Cut(plan, "-")
	if !ok {
		return "", "", fmt.Errorf("invalid OpenSearch plan: %s", plan)
	}

	tier, ok := aivenPlans[t]
	if !ok {
		return "", "", fmt.Errorf("invalid OpenSearch tier: %s", t)
	}

	size, ok := aivenSizes[s]
	if !ok {
		return "", "", fmt.Errorf("invalid OpenSearch size: %s", s)
	}

	return tier, size, nil
}
