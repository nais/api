// Package agent provides the AI chat service for the Nais platform.
package agent

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/pkg/mcp"
	"github.com/nais/api/pkg/mcp/tools"
	"github.com/sirupsen/logrus"
)

// MCPIntegration provides MCP tool execution capabilities for the agent.
// It wraps the pkg/mcp.Executor and provides access to schema exploration
// and GraphQL execution tools.
type MCPIntegration struct {
	executor *mcp.Executor
	client   *InternalClient
	config   MCPIntegrationConfig
	log      logrus.FieldLogger
}

// MCPIntegrationConfig holds configuration for the MCP integration.
type MCPIntegrationConfig struct {
	// Handler is the gqlgen handler server for executing GraphQL queries.
	Handler *handler.Server

	// TenantName is the tenant name for building console URLs.
	TenantName string

	// Log is the logger for operations.
	Log logrus.FieldLogger
}

// NewMCPIntegration creates a new MCP integration for the agent.
func NewMCPIntegration(cfg MCPIntegrationConfig) (*MCPIntegration, error) {
	if cfg.Handler == nil {
		return nil, fmt.Errorf("handler is required")
	}

	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}

	// Get the schema from gengql
	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()

	// Create the internal client
	internalClient := NewInternalClient(InternalClientConfig{
		Handler: cfg.Handler,
		Log:     cfg.Log.WithField("component", "mcp_client"),
	})

	// Create the mcp.Client adapter that wraps our InternalClient
	mcpClient := &internalClientAdapter{client: internalClient}

	// Create the schema provider
	schemaProvider := mcp.NewStaticSchemaProvider(schema)

	// Create the MCP executor using pkg/mcp
	executor, err := mcp.NewExecutor(
		mcp.WithClient(mcpClient),
		mcp.WithSchemaProvider(schemaProvider),
		mcp.WithTenantName(cfg.TenantName),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP executor: %w", err)
	}

	return &MCPIntegration{
		executor: executor,
		client:   internalClient,
		config:   cfg,
		log:      cfg.Log,
	}, nil
}

// ExecuteTool executes an MCP tool by name with the given input.
func (m *MCPIntegration) ExecuteTool(ctx context.Context, name string, input map[string]any) (any, error) {
	return m.executor.ExecuteTool(ctx, name, input)
}

// ListTools returns all available tool definitions from the MCP executor.
func (m *MCPIntegration) ListTools() []tools.ToolDefinition {
	return m.executor.ListTools()
}

// Client returns the underlying internal client.
func (m *MCPIntegration) Client() *InternalClient {
	return m.client
}

// Registry returns the underlying tool registry.
// This provides direct access to the unified tool registry for advanced use cases.
func (m *MCPIntegration) Registry() *tools.Registry {
	return m.executor.Registry()
}

// internalClientAdapter adapts InternalClient to the mcp.Client interface.
type internalClientAdapter struct {
	client *InternalClient
}

func (a *internalClientAdapter) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	return a.client.ExecuteGraphQL(ctx, query, variables)
}

func (a *internalClientAdapter) GetCurrentUser(ctx context.Context) (*mcp.UserInfo, error) {
	user, err := a.client.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return &mcp.UserInfo{
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
	}, nil
}

func (a *internalClientAdapter) GetUserTeams(ctx context.Context) ([]mcp.TeamInfo, error) {
	teams, err := a.client.GetUserTeams(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]mcp.TeamInfo, len(teams))
	for i, t := range teams {
		result[i] = mcp.TeamInfo{
			Slug:    t.Slug,
			Purpose: t.Purpose,
			Role:    t.Role,
		}
	}
	return result, nil
}

// Ensure internalClientAdapter implements mcp.Client
var _ mcp.Client = (*internalClientAdapter)(nil)
