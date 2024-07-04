package application

import (
	"context"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
	"slices"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *ApplicationOrder) (*ApplicationConnection, error) {
	k8s := fromContext(ctx).k8sClient

	allApplications, err := k8s.Apps(ctx, teamSlug.String())
	if err != nil {
		return nil, err
	}

	if orderBy != nil {
		switch orderBy.Field {
		case ApplicationOrderFieldName:
			slices.SortStableFunc(allApplications, func(a, b *model.App) int {
				return modelv1.Compare(a.Name, b.Name, orderBy.Direction)
			})
		case ApplicationOrderFieldEnvironment:
			slices.SortStableFunc(allApplications, func(a, b *model.App) int {
				return modelv1.Compare(a.Env.Name, b.Env.Name, orderBy.Direction)
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
	}

	apps := pagination.Slice(allApplications, page)
	return pagination.NewConvertConnection(apps, page, int32(len(allApplications)), toGraphApplication), nil
}
