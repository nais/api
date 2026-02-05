package mcp

import (
	"context"
)

// Client abstracts GraphQL operations and authentication for MCP tools.
// Implementations handle the actual GraphQL execution, whether via HTTP
// to an external API or direct execution against a local schema.
type Client interface {
	// ExecuteGraphQL runs a GraphQL query with the given variables.
	// Returns the data portion of the response as a map.
	ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error)

	// GetCurrentUser returns information about the current authenticated user.
	GetCurrentUser(ctx context.Context) (*UserInfo, error)

	// GetUserTeams returns the teams the current user belongs to.
	GetUserTeams(ctx context.Context) ([]TeamInfo, error)
}

// UserInfo contains information about an authenticated user.
type UserInfo struct {
	// Name is the user's display name or email.
	Name string

	// IsAdmin indicates whether the user has admin privileges.
	IsAdmin bool
}

// TeamInfo contains information about a team the user belongs to.
type TeamInfo struct {
	// Slug is the team's unique identifier.
	Slug string

	// Purpose describes what the team does.
	Purpose string

	// Role is the user's role in the team (e.g., "owner", "member").
	Role string
}
