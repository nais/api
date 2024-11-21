package repository

import (
	"context"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/github/repository/repositorysql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
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
	return pagination.NewConvertConnection(ret, page, total, toGraphRepository), nil
}

func AddToTeam(ctx context.Context, input AddRepositoryToTeamInput) (*Repository, error) {
	var tr *repositorysql.TeamRepository
	err := database.Transaction(ctx, func(ctx context.Context) error {
		var err error
		tr, err = db(ctx).AddToTeam(ctx, repositorysql.AddToTeamParams{
			TeamSlug:         input.TeamSlug,
			GithubRepository: input.RepositoryName,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateInput{
			Action:       audit.AuditActionAdded,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: auditResourceTypeRepository,
			ResourceName: input.RepositoryName,
			TeamSlug:     ptr.To(input.TeamSlug),
		})
	})
	if err != nil {
		return nil, err
	}

	return toGraphRepository(tr), nil
}

func RemoveFromTeam(ctx context.Context, input RemoveRepositoryFromTeamInput) error {
	return database.Transaction(ctx, func(ctx context.Context) error {
		err := db(ctx).RemoveFromTeam(ctx, repositorysql.RemoveFromTeamParams{
			TeamSlug:         input.TeamSlug,
			GithubRepository: input.RepositoryName,
		})
		if err != nil {
			return err
		}

		return audit.Create(ctx, audit.CreateInput{
			Action:       audit.AuditActionRemoved,
			Actor:        authz.ActorFromContext(ctx).User,
			ResourceType: auditResourceTypeRepository,
			ResourceName: input.RepositoryName,
			TeamSlug:     ptr.To(input.TeamSlug),
		})
	})
}
