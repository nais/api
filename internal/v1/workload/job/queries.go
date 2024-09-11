package job

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *JobOrder) (*JobConnection, error) {
	allJobs := fromContext(ctx).jobWatcher.GetByNamespace(teamSlug.String())
	ret := make([]*Job, len(allJobs))
	for i, obj := range allJobs {
		ret[i] = toGraphJob(obj.Obj, obj.Cluster)
	}

	if orderBy != nil {
		switch orderBy.Field {
		case JobOrderFieldName:
			slices.SortStableFunc(ret, func(a, b *Job) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case JobOrderFieldEnvironment:
			slices.SortStableFunc(ret, func(a, b *Job) int {
				return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
			})
		case JobOrderFieldDeploymentTime:
			panic("not implemented yet")
		case JobOrderFieldStatus:
			panic("not implemented yet")
		}
	}

	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, int32(len(allJobs))), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Job, error) {
	job, err := fromContext(ctx).jobWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphJob(job, environment), nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Job, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}
