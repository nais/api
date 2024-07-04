package team

import (
	"context"

	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"
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

func ListMembers(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *TeamMemberOrder) (*TeamMemberConnection, error) {
	db := fromContext(ctx).db

	ret, err := db.ListMembers(ctx, teamsql.ListMembersParams{
		TeamSlug: teamSlug,
		Offset:   page.Offset(),
		Limit:    page.Limit(),
		OrderBy:  orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := db.CountMembers(ctx, &teamSlug)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphTeamMember), nil
}

func GetTeamEnvironment(ctx context.Context, teamSlug slug.Slug, envName string) (*TeamEnvironment, error) {
	return fromContext(ctx).teamEnvironmentLoader.Load(ctx, envSlugName{Slug: teamSlug, EnvName: envName})
}
