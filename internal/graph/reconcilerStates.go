package graph

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
)

// TODO: How to do this?
type GitHubRepositoryPermission struct {
	Name    string `json:"name"`
	Granted bool   `json:"granted"`
}

type GitHubRepository struct {
	Name        string                        `json:"name"`
	Permissions []*GitHubRepositoryPermission `json:"permissions"`
	Archived    bool                          `json:"archived"`
	RoleName    string                        `json:"roleName"`
	TeamSlug    *slug.Slug                    `json:"-"`
}

type GitHubState struct {
	Slug         *slug.Slug          `json:"slug"`
	Repositories []*GitHubRepository `json:"repositories"`
}

type GoogleWorkspaceState struct {
	GroupEmail *string `json:"groupEmail"`
}

type GoogleGcpEnvironmentProject struct {
	ProjectID string `json:"projectId"` // Unique of the project, for instance `my-project-123`
}
type GoogleGcpProjectState struct {
	Projects map[string]GoogleGcpEnvironmentProject `json:"projects"` // environment name is used as key
}

type NaisNamespaceState struct {
	Namespaces map[string]slug.Slug `json:"namespaces"` // Key is the environment for the team namespace
}

type AzureState struct {
	GroupID uuid.UUID `json:"groupId"`
}

type NaisDeployKeyState struct {
	Provisioned *time.Time `json:"provisioned"`
}

type GoogleGarState struct {
	RepositoryName *string `json:"repopsitoryName"`
}

func toGraphGithubRepository(m *GitHubRepository) *model.GitHubRepository {
	ret := &model.GitHubRepository{
		Name:     m.Name,
		RoleName: m.RoleName,
		Archived: m.Archived,
	}

	for _, p := range m.Permissions {
		ret.Permissions = append(ret.Permissions, &model.GitHubRepositoryPermission{
			Name:    p.Name,
			Granted: p.Granted,
		})
	}

	return ret
}
