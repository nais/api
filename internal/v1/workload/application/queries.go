package application

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ApplicationOrder) (*ApplicationConnection, error) {
	k8s := fromContext(ctx).appWatcher

	allApplications := k8s.GetByNamespace(teamSlug.String())

	ret := make([]*Application, len(allApplications))
	for i, obj := range allApplications {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster)
	}

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
	case ApplicationOrderFieldVulnerabilities:
		panic("not implemented yet")
	case ApplicationOrderFieldRiskScore:
		panic("not implemented yet")
	case ApplicationOrderFieldDeploymentTime:
		panic("not implemented yet")
	case ApplicationOrderFieldStatus:
		panic("not implemented yet")
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, int32(len(allApplications))), nil
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Application, error) {
	return fromContext(ctx).applicationLoader.Load(ctx, applicationIdentifier{
		namespace:   teamSlug.String(),
		environment: environment,
		name:        name,
	})
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Application, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}
