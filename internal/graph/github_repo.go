package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func toGraphGitHubRepositories(teamSlug slug.Slug, state *database.ReconcilerState, filter *model.GitHubRepositoriesFilter) ([]*model.GitHubRepository, error) {
	repos, err := database.GetGitHubRepos(state.Value)
	if err != nil {
		return nil, err
	}

	excludeArchived := filter == nil || filter.IncludeArchivedRepositories == nil || !*filter.IncludeArchivedRepositories

	ret := make([]*model.GitHubRepository, 0)
	for _, repo := range repos {
		if repo.Archived && excludeArchived {
			continue
		}
		ret = append(ret, toGraphGitHubRepository(teamSlug, repo))
	}

	return ret, nil
}

func toGraphGitHubRepository(teamSlug slug.Slug, repo *database.GitHubRepository) *model.GitHubRepository {
	return &model.GitHubRepository{
		ID:       scalar.GitHubRepository(repo.Name),
		Name:     repo.Name,
		RoleName: repo.RoleName,
		Archived: repo.Archived,
		Permissions: func(perms []*database.GitHubRepositoryPermission) []*model.GitHubRepositoryPermission {
			ret := make([]*model.GitHubRepositoryPermission, len(perms))
			for i, perm := range repo.Permissions {
				ret[i] = &model.GitHubRepositoryPermission{
					Name:    perm.Name,
					Granted: perm.Granted,
				}
			}
			return ret
		}(repo.Permissions),
		GQLVars: model.GitHubRepositoryGQLVars{
			TeamSlug: teamSlug,
		},
	}
}
