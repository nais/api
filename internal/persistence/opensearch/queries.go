package opensearch

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
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

func Create(ctx context.Context, input *CreateOpenSearchInput) (*CreateOpenSearchPayload, error) {
	client, err := fromContext(ctx).watcher.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, err
	}

	plan, err := createPlanID(input)
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

	aivenProject := fromContext(ctx).aivenProjects[input.EnvironmentName]

	res.Object["spec"] = map[string]any{
		"cloudName":             "google-europe-north1",
		"plan":                  plan,
		"project":               aivenProject.ID,
		"projectVpcId":          aivenProject.VPC,
		"terminationProtection": true,
		"tags": map[string]string{
			"environment": input.EnvironmentName,
			"team":        input.TeamSlug.String(),
			"tenant":      fromContext(ctx).tenantName,
		},
	}

	ret, err := client.Create(ctx, res, metav1.CreateOptions{})
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

func createPlanID(input *CreateOpenSearchInput) (string, error) {
	plan := ""

	switch input.Tier {
	case OpenSearchTierHighAvailability:
		plan = "business-"
	case OpenSearchTierSingleNode:
		plan = "startup-"
	default:
		return "", apierror.Errorf("invalid OpenSearch tier: %s", input.Tier)
	}

	switch input.Size {
	case OpenSearchSizeRAM8gb:
		plan += "8"
	case OpenSearchSizeRAM16gb:
		plan += "16"
	case OpenSearchSizeRAM32gb:
		plan += "32"
	case OpenSearchSizeRAM64gb:
		plan += "64"
	default:
		return "", apierror.Errorf("invalid OpenSearch size: %s", input.Size)
	}

	return plan, nil
}
