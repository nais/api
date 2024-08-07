package application

import (
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ApplicationOrder) (*ApplicationConnection, error) {
	k8s := fromContext(ctx).appWatcher

	allApplications := k8s.GetByNamespace(teamSlug.String())

	if orderBy == nil {
		orderBy = &ApplicationOrder{
			Field:     ApplicationOrderFieldName,
			Direction: modelv1.OrderDirectionAsc,
		}
	}

	switch orderBy.Field {
	case ApplicationOrderFieldName:
		slices.SortStableFunc(allApplications, func(a, b *nais_io_v1alpha1.Application) int {
			return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
		})
	case ApplicationOrderFieldEnvironment:
		// slices.SortStableFunc(allApplications, func(a, b *nais_io_v1alpha1.Application) int {
		// 	return modelv1.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
		// })
	case ApplicationOrderFieldVulnerabilities:
		panic("not implemented yet")
	case ApplicationOrderFieldRiskScore:
		panic("not implemented yet")
	case ApplicationOrderFieldDeploymentTime:
		panic("not implemented yet")
	case ApplicationOrderFieldStatus:
		panic("not implemented yet")
	}

	apps := pagination.Slice(allApplications, page)
	return pagination.NewConvertConnection(apps, page, int32(len(allApplications)), toGraphApplication), nil
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
