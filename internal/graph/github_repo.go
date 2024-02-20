package graph

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
)

func toGraphGitHubRepositories(r *database.ReconcilerState) ([]*model.GitHubRepository, error) {
	repos, err := database.GetGitHubRepos(r.Value)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.GitHubRepository, len(repos))
	for i, repo := range repos {
		ret[i] = toGraphGitHubRepository(repo)
	}

	return ret, nil
}

func toGraphGitHubRepository(repo *database.GitHubRepository) *model.GitHubRepository {
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
	}
}
