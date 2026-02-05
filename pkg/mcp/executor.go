package mcp

import (
	"context"

	"github.com/nais/api/pkg/mcp/tools"
)

// Executor provides direct tool execution without MCP protocol overhead.
// This is the primary interface for the hosted agent to invoke tools.
type Executor struct {
	registry *tools.Registry
	config   *Config
}

// NewExecutor creates a new tool executor for direct invocation.
// Use this for the hosted agent use case where MCP protocol is not needed.
func NewExecutor(opts ...Option) (*Executor, error) {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	if config.Client == nil {
		return nil, ErrClientRequired
	}
	if config.SchemaProvider == nil {
		return nil, ErrSchemaProviderRequired
	}

	// Create the underlying tool registry
	registry := tools.NewRegistry(tools.RegistryConfig{
		Client:             &clientAdapter{config.Client},
		Schema:             config.SchemaProvider.GetSchema(),
		ConsoleBaseURL:     config.ConsoleBaseURL(),
		ConsoleURLPatterns: config.ConsoleURLPatterns(),
		Logger:             config.Logger,
	})

	return &Executor{
		registry: registry,
		config:   config,
	}, nil
}

// ExecuteTool runs a tool by name with the given input.
// This is the main method for the hosted agent to call tools directly.
//
// Example:
//
//	result, err := executor.ExecuteTool(ctx, "execute_graphql", map[string]any{
//	    "query": `query { team(slug: "my-team") { slug } }`,
//	})
func (e *Executor) ExecuteTool(ctx context.Context, name string, input map[string]any) (any, error) {
	return e.registry.Execute(ctx, name, input)
}

// ListTools returns all available tool definitions.
func (e *Executor) ListTools() []tools.ToolDefinition {
	return e.registry.GetToolDefinitions()
}

// Registry returns the underlying tool registry.
// This provides direct access to the unified tool registry for advanced use cases.
func (e *Executor) Registry() *tools.Registry {
	return e.registry
}

// clientAdapter adapts the mcp.Client interface to tools.GraphQLClient.
type clientAdapter struct {
	client Client
}

func (a *clientAdapter) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	return a.client.ExecuteGraphQL(ctx, query, variables)
}

func (a *clientAdapter) GetCurrentUser(ctx context.Context) (*tools.UserInfo, error) {
	user, err := a.client.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return &tools.UserInfo{
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
	}, nil
}

func (a *clientAdapter) GetUserTeams(ctx context.Context) ([]tools.TeamInfo, error) {
	teams, err := a.client.GetUserTeams(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]tools.TeamInfo, len(teams))
	for i, t := range teams {
		result[i] = tools.TeamInfo{
			Slug:    t.Slug,
			Purpose: t.Purpose,
			Role:    t.Role,
		}
	}
	return result, nil
}
