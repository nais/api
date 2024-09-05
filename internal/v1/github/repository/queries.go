package repository

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/github/repository/repositorysql"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

// func AssignTeamRoleToUser(ctx context.Context, userID uuid.UUID, teamSlug slug.Slug, roleName rolesql.RoleName) error {
// 	return db(ctx).AssignTeamRoleToUser(ctx, rolesql.AssignTeamRoleToUserParams{
// 		UserID:         userID,
// 		RoleName:       roleName,
// 		TargetTeamSlug: teamSlug,
// 	})
// }

func getByIdent(_ context.Context, id ident.Ident) (*Repository, error) {
	ts, repo, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	return &Repository{
		TeamSlug: ts,
		Name:     repo,
	}, nil
}

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*RepositoryConnection, error) {
	q := db(ctx)

	ret, err := q.ListForTeam(ctx, repositorysql.ListForTeamParams{
		TeamSlug: teamSlug,
		Offset:   page.Offset(),
		Limit:    page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphRepository), nil
}

func Create(ctx context.Context, input AddRepositoryToTeamInput) (*Repository, error) {
	q := db(ctx)

	ret, err := q.Create(ctx, repositorysql.CreateParams{
		TeamSlug:         input.TeamSlug,
		GithubRepository: input.RepoName,
	})
	if err != nil {
		return nil, err
	}

	return toGraphRepository(ret), nil
}

func Remove(ctx context.Context, input RemoveRepositoryFromTeamInput) (*Repository, error) {
	q := db(ctx)

	ret, err := q.Remove(ctx, repositorysql.RemoveParams{
		TeamSlug:         input.TeamSlug,
		GithubRepository: input.RepoName,
	})
	if err != nil {
		return nil, err
	}

	return toGraphRepository(ret), nil
}
