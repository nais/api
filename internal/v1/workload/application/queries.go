package application

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/searchv1"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ApplicationOrder) (*ApplicationConnection, error) {
	ret := ListAllForTeam(ctx, teamSlug)

	if orderBy == nil {
		orderBy = &ApplicationOrder{
			Field:     ApplicationOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}

	switch orderBy.Field {
	case ApplicationOrderFieldName:
		slices.SortStableFunc(ret, func(a, b *Application) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	case ApplicationOrderFieldEnvironment:
		slices.SortStableFunc(ret, func(a, b *Application) int {
			return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
		})
	case ApplicationOrderFieldDeploymentTime:
		slices.SortStableFunc(ret, func(a, b *Application) int {
			return modelv1.Compare(a.EnvironmentName, b.EnvironmentName, orderBy.Direction)
		})
	case ApplicationOrderFieldStatus:
		panic("not implemented yet")
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, int32(len(ret))), nil
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Application {
	k8s := fromContext(ctx).appWatcher

	allApplications := k8s.GetByNamespace(teamSlug.String())

	ret := make([]*Application, len(allApplications))
	for i, obj := range allApplications {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster)
	}
	return ret
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Application, error) {
	a, err := fromContext(ctx).appWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphApplication(a, environment), nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Application, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	apps := fromContext(ctx).appWatcher.All()

	ret := make([]*searchv1.Result, 0)
	for _, app := range apps {
		rank := searchv1.Match(q, app.Obj.Name)
		if searchv1.Include(rank) {
			ret = append(ret, &searchv1.Result{
				Rank: rank,
				Node: toGraphApplication(app.Obj, app.Cluster),
			})
		}
	}

	return ret, nil
}
