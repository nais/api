package graph

import (
	"encoding/json"
	"sort"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
)

type gitHubState struct {
	Repositories []*gitHubRepository `json:"repositories"`
}

type gitHubRepository struct {
	Name        string                        `json:"name"`
	Permissions []*gitHubRepositoryPermission `json:"permissions"`
	Archived    bool                          `json:"archived"`
	RoleName    string                        `json:"roleName"`
}

type gitHubRepositoryPermission struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
}

func getGitHubRepos(b []byte) ([]*gitHubRepository, error) {
	var repos gitHubState
	err := json.Unmarshal(b, &repos)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(repos.Repositories, func(i, j int) bool {
		return repos.Repositories[i].Name < repos.Repositories[j].Name
	})
	return repos.Repositories, nil
}

func toGraphGitHubRepositories(r *database.ReconcilerState) ([]*model.GitHubRepository, error) {
	repos, err := getGitHubRepos(r.Value)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.GitHubRepository, len(repos))
	for i, repo := range repos {
		ret[i] = toGraphGitHubRepository(repo)
	}

	return ret, nil
}

func toGraphGitHubRepository(repo *gitHubRepository) *model.GitHubRepository {
	return &model.GitHubRepository{
		Name:     repo.Name,
		RoleName: repo.RoleName,
		Archived: repo.Archived,
		Permissions: func(perms []*gitHubRepositoryPermission) []*model.GitHubRepositoryPermission {
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
