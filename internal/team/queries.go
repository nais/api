package team

import (
	"context"

	"github.com/nais/api/internal/team/teamsql"

	"github.com/nais/api/internal/slug"

	"github.com/nais/api/internal/graphv1/pagination"
)

func Get(ctx context.Context, slug slug.Slug) (*Team, error) {
	return fromContext(ctx).teamLoader.Load(ctx, slug)
}

func List(ctx context.Context, page *pagination.Pagination, orderBy *TeamOrder) (*TeamConnection, error) {
	db := fromContext(ctx).db

	ret, err := db.List(ctx, teamsql.ListParams{
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := db.Count(ctx)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphTeam), nil
}
