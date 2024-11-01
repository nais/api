package repository

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/github/repository/repositorysql"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

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

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination, orderBy *RepositoryOrder, filter *TeamRepositoryFilter) (*RepositoryConnection, error) {
	if filter == nil {
		filter = &TeamRepositoryFilter{}
	}

	q := db(ctx)

	ret, err := q.ListForTeam(ctx, repositorysql.ListForTeamParams{
		TeamSlug: teamSlug,
		Offset:   page.Offset(),
		Limit:    page.Limit(),
		Search:   filter.Name,
		OrderBy:  orderBy.String(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountForTeam(ctx, repositorysql.CountForTeamParams{
		TeamSlug: teamSlug,
		Search:   filter.Name,
	})
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnection(ret, page, int32(total), toGraphRepository), nil
}

func Create(ctx context.Context, input AddRepositoryToTeamInput) (*Repository, error) {
	q := db(ctx)

	ret, err := q.Create(ctx, repositorysql.CreateParams{
		TeamSlug:         input.TeamSlug,
		GithubRepository: input.RepositoryName,
	})
	if err != nil {
		return nil, err
	}

	return toGraphRepository(ret), nil
}

func Remove(ctx context.Context, input RemoveRepositoryFromTeamInput) error {
	return db(ctx).Remove(ctx, repositorysql.RemoveParams{
		TeamSlug:         input.TeamSlug,
		GithubRepository: input.RepositoryName,
	})
}
