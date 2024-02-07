package graph

import (
	"encoding/json"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
)

type gitHubRepository struct {
	Permissions []*gitHubRepositoryPermission `json:"permissions"`
	Archived    bool                          `json:"archived"`
	RoleName    string                        `json:"roleName"`
}

type gitHubRepositoryPermission struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
}

func parseGHRepoMetadata(b []byte) (*gitHubRepository, error) {
	var repo gitHubRepository
	err := json.Unmarshal(b, &repo)
	if err != nil {
		return nil, err
	}
	return &repo, nil
}

func toGraphGitHubRepository(r *database.ReconcilerResource) (*model.GitHubRepository, error) {
	meta, err := parseGHRepoMetadata(r.Metadata)
	if err != nil {
		return nil, err
	}
	return &model.GitHubRepository{
		ID:       r.ID,
		Name:     r.Value,
		RoleName: meta.RoleName,
		Archived: meta.Archived,
		Permissions: func() []*model.GitHubRepositoryPermission {
			perms := make([]*model.GitHubRepositoryPermission, 0, len(meta.Permissions))
			for _, perm := range meta.Permissions {
				perms = append(perms, &model.GitHubRepositoryPermission{
					Name:    perm.Name,
					Granted: perm.Granted,
				})
			}
			return perms
		}(),
	}, nil
}
