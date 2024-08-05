package team

import (
	"context"

	"github.com/google/uuid"

	"github.com/nais/api/internal/v1/graphv1/ident"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team/teamsql"
)

func Get(ctx context.Context, slug slug.Slug) (*Team, error) {
	return fromContext(ctx).teamLoader.Load(ctx, slug)
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Team, error) {
	teamSlug, err := parseTeamIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug)
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

func ListForUser(ctx context.Context, userID uuid.UUID, page *pagination.Pagination, orderBy *TeamMembershipOrder) (*TeamMemberConnection, error) {
	db := fromContext(ctx).db

	ret, err := db.ListForUser(ctx, teamsql.ListForUserParams{
		UserID:  userID,
		Offset:  page.Offset(),
		Limit:   page.Limit(),
		OrderBy: orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := db.CountForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphUserTeam), nil
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

func GetTeamEnvironmentByIdent(ctx context.Context, id ident.Ident) (*TeamEnvironment, error) {
	teamSlug, envName, err := parseTeamEnvironmentIdent(id)
	if err != nil {
		return nil, err
	}
	return GetTeamEnvironment(ctx, teamSlug, envName)
}
