package graph

import (
	"context"
	"errors"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

func (r *gitHubRepositoryResolver) Authorizations(ctx context.Context, obj *model.GitHubRepository) ([]model.RepositoryAuthorization, error) {
	authz, err := r.database.GetRepositoryAuthorizations(ctx, obj.GQLVars.TeamSlug, obj.Name)
	if err != nil {
		return nil, err
	}

	var ret []model.RepositoryAuthorization
	for _, a := range authz {
		switch a {
		default:
			return nil, fmt.Errorf("invalid authorization: %q", string(a))
		case gensql.RepositoryAuthorizationEnumDeploy:
			ret = append(ret, model.RepositoryAuthorizationDeploy)
		}
	}

	return ret, nil
}

func (r *mutationResolver) AuthorizeRepository(ctx context.Context, authorization model.RepositoryAuthorization, teamSlug slug.Slug, repoName string) (*model.GitHubRepository, error) {
	actor := authz.ActorFromContext(ctx)
	if _, err := r.database.GetTeamMember(ctx, teamSlug, actor.User.GetID()); errors.Is(err, pgx.ErrNoRows) {
		return nil, apierror.ErrUserIsNotTeamMember
	} else if err != nil {
		return nil, err
	}

	var repoAuthorization gensql.RepositoryAuthorizationEnum
	switch authorization {
	default:
		return nil, fmt.Errorf("invalid authorization: %q", string(authorization))
	case model.RepositoryAuthorizationDeploy:
		repoAuthorization = gensql.RepositoryAuthorizationEnumDeploy
	}

	state, err := r.database.GetReconcilerStateForTeam(ctx, "github:team", teamSlug)
	if err != nil {
		return nil, err
	}

	var repo *model.GitHubRepository
	repos, err := database.GetGitHubRepos(state.Value)
	if err != nil {
		return nil, err
	}

	for _, r := range repos {
		if r.Name == repoName {
			repo = toGraphGitHubRepository(teamSlug, r)
			break
		}
	}
	if repo == nil {
		return nil, apierror.Errorf("Repository %q not present in the team state", repoName)
	}

	if err := r.database.CreateRepositoryAuthorization(ctx, teamSlug, repoName, repoAuthorization); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *mutationResolver) DeauthorizeRepository(ctx context.Context, authorization model.RepositoryAuthorization, teamSlug slug.Slug, repoName string) (*model.GitHubRepository, error) {
	actor := authz.ActorFromContext(ctx)
	if _, err := r.database.GetTeamMember(ctx, teamSlug, actor.User.GetID()); errors.Is(err, pgx.ErrNoRows) {
		return nil, apierror.ErrUserIsNotTeamMember
	} else if err != nil {
		return nil, err
	}

	var repoAuthorization gensql.RepositoryAuthorizationEnum
	switch authorization {
	default:
		return nil, fmt.Errorf("invalid authorization: %q", string(authorization))
	case model.RepositoryAuthorizationDeploy:
		repoAuthorization = gensql.RepositoryAuthorizationEnumDeploy
	}

	state, err := r.database.GetReconcilerStateForTeam(ctx, "github:team", teamSlug)
	if err != nil {
		return nil, err
	}

	var repo *model.GitHubRepository
	repos, err := database.GetGitHubRepos(state.Value)
	if err != nil {
		return nil, err
	}

	for _, r := range repos {
		if r.Name == repoName {
			repo = toGraphGitHubRepository(teamSlug, r)
			break
		}
	}
	if repo == nil {
		return nil, apierror.Errorf("Repository %q not present in the team state", repoName)
	}

	if err := r.database.RemoveRepositoryAuthorization(ctx, teamSlug, repoName, repoAuthorization); err != nil {
		return nil, err
	}

	return repo, nil
}

func (r *Resolver) GitHubRepository() gengql.GitHubRepositoryResolver {
	return &gitHubRepositoryResolver{r}
}

type gitHubRepositoryResolver struct{ *Resolver }
