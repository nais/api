package opensearch

import (
	"context"
	"fmt"
	"strings"

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
)

func GetByIdent(ctx context.Context, id ident.Ident) (*OpenSearch, error) {
	teamSlug, environment, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return Get(ctx, teamSlug, environment, name)
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*OpenSearch, error) {
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

func GetOpenSearchVersion(ctx context.Context, key AivenDataLoaderKey) (string, error) {
	return fromContext(ctx).versionLoader.Load(ctx, &key)
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

	plan, err := aivenPlan(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}

	res := &unstructured.Unstructured{}
	res.SetAPIVersion("aiven.io/v1alpha1")
	res.SetKind("OpenSearch")
	res.SetName(openSearchNamer(input.TeamSlug, input.Name))
	res.SetNamespace(input.TeamSlug.String())
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	aivenProject, err := aiven.GetProject(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	res.Object["spec"] = map[string]any{
		"cloudName":             "google-europe-north1",
		"plan":                  plan,
		"project":               aivenProject.ID,
		"projectVpcId":          aivenProject.VPC,
		"terminationProtection": true,
		"tags": map[string]any{
			"environment": input.EnvironmentName,
			"team":        input.TeamSlug.String(),
			"tenant":      fromContext(ctx).tenantName,
		},
	}
	if input.Version != nil {
		version := strings.TrimLeft(input.Version.String(), "V")
		err := unstructured.SetNestedField(res.Object, version, "spec", "userConfig", "opensearch_version")
		if err != nil {
			return nil, err
		}
	}

	ret, err := client.Namespace(input.TeamSlug.String()).Create(ctx, res, metav1.CreateOptions{})
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return nil, apierror.ErrAlreadyExists
		}
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

	openSearch, err := client.Namespace(input.TeamSlug.String()).Get(ctx, openSearchNamer(input.TeamSlug, input.Name), metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !kubernetes.HasManagedByConsoleLabel(openSearch) {
		return nil, apierror.Errorf("OpenSearch %s/%s is not managed by Console", input.TeamSlug, input.Name)
	}

	plan, err := aivenPlan(input.Tier, input.Size)
	if err != nil {
		return nil, err
	}
	err = unstructured.SetNestedField(openSearch.Object, plan, "spec", "plan")
	if err != nil {
		return nil, err
	}

	if input.Version != nil {
		version := strings.TrimLeft(input.Version.String(), "V")
		err = unstructured.SetNestedField(openSearch.Object, version, "spec", "userConfig", "opensearch_version")
		if err != nil {
			return nil, err
		}
	}

	ret, err := client.Namespace(input.TeamSlug.String()).Update(ctx, openSearch, metav1.UpdateOptions{})
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

func aivenPlan(tier OpenSearchTier, size OpenSearchSize) (string, error) {
	plan := ""

	switch tier {
	case OpenSearchTierHighAvailability:
		plan = "business-"
	case OpenSearchTierSingleNode:
		plan = "startup-"
	default:
		return "", apierror.Errorf("invalid OpenSearch tier: %s", tier)
	}

	switch size {
	case OpenSearchSizeRAM4gb:
		plan += "4"
	case OpenSearchSizeRAM8gb:
		plan += "8"
	case OpenSearchSizeRAM16gb:
		plan += "16"
	case OpenSearchSizeRAM32gb:
		plan += "32"
	case OpenSearchSizeRAM64gb:
		plan += "64"
	default:
		return "", apierror.Errorf("invalid OpenSearch size: %s", size)
	}

	return plan, nil
}

var aivenPlans = map[string]OpenSearchTier{
	"business": OpenSearchTierHighAvailability,
	"startup":  OpenSearchTierSingleNode,
}

var aivenSizes = map[string]OpenSearchSize{
	"4":  OpenSearchSizeRAM4gb,
	"8":  OpenSearchSizeRAM8gb,
	"16": OpenSearchSizeRAM16gb,
	"32": OpenSearchSizeRAM32gb,
	"64": OpenSearchSizeRAM64gb,
}

func tierAndSizeFromPlan(plan string) (OpenSearchTier, OpenSearchSize, error) {
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
