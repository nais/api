package job

import (
	"context"
	"slices"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *JobOrder) (*JobConnection, error) {
	k8s := fromContext(ctx).k8sClient

	allJobs, err := k8s.NaisJobs(ctx, teamSlug.String())
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case JobOrderFieldName:
			slices.SortStableFunc(allJobs, func(a, b *model.NaisJob) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case JobOrderFieldEnvironment:
			slices.SortStableFunc(allJobs, func(a, b *model.NaisJob) int {
				return modelv1.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
			})
		case JobOrderFieldVulnerabilities:
			panic("not implemented yet")
		case JobOrderFieldRiskScore:
			panic("not implemented yet")
		case JobOrderFieldDeploymentTime:
			panic("not implemented yet")
		case JobOrderFieldStatus:
			panic("not implemented yet")
		}
	}

	jobs := pagination.Slice(allJobs, page)
	return pagination.NewConvertConnection(jobs, page, int32(len(allJobs)), toGraphJob), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Job, error) {
	return fromContext(ctx).jobLoader.Load(ctx, jobIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Job, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}
