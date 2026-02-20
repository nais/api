// Package agent provides the AI chat service for the Nais platform.
package agent

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/nais/api/internal/agent/tools"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/sirupsen/logrus"
)

// ToolIntegration provides tool execution capabilities for the agent.
// It wraps the tools.Registry and provides access to schema exploration
// and GraphQL execution tools.
type ToolIntegration struct {
	registry *tools.Registry
	client   *InternalClient
	config   ToolIntegrationConfig
	log      logrus.FieldLogger
}

// ToolIntegrationConfig holds configuration for the tool integration.
type ToolIntegrationConfig struct {
	// Handler is the gqlgen handler server for executing GraphQL queries.
	Handler *handler.Server

	// TenantName is the tenant name for building console URLs.
	TenantName string

	// Log is the logger for operations.
	Log logrus.FieldLogger
}

// NewToolIntegration creates a new tool integration for the agent.
func NewToolIntegration(cfg ToolIntegrationConfig) (*ToolIntegration, error) {
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
		Log:     cfg.Log.WithField("component", "tools_client"),
	})

	// Create the GraphQL client adapter
	graphqlClient := &graphqlClientAdapter{client: internalClient}

	// Build console URL patterns
	consoleBaseURL, urlPatterns := buildConsoleURLs(cfg.TenantName)

	// Create the tool registry
	registry := tools.NewRegistry(tools.RegistryConfig{
		Client:             graphqlClient,
		Schema:             schema,
		ConsoleBaseURL:     consoleBaseURL,
		ConsoleURLPatterns: urlPatterns,
	})

	return &ToolIntegration{
		registry: registry,
		client:   internalClient,
		config:   cfg,
		log:      cfg.Log,
	}, nil
}

// ExecuteTool executes a tool by name with the given input.
func (t *ToolIntegration) ExecuteTool(ctx context.Context, name string, input map[string]any) (any, error) {
	return t.registry.Execute(ctx, name, input)
}

// ListTools returns all available tool definitions.
func (t *ToolIntegration) ListTools() []tools.Tool {
	return t.registry.ListTools()
}

// Client returns the underlying internal client.
func (t *ToolIntegration) Client() *InternalClient {
	return t.client
}

// graphqlClientAdapter adapts InternalClient to the tools.GraphQLClient interface.
type graphqlClientAdapter struct {
	client *InternalClient
}

func (a *graphqlClientAdapter) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	return a.client.ExecuteGraphQL(ctx, query, variables)
}

func (a *graphqlClientAdapter) GetCurrentUser(ctx context.Context) (*tools.UserInfo, error) {
	user, err := a.client.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	return &tools.UserInfo{
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
	}, nil
}

func (a *graphqlClientAdapter) GetUserTeams(ctx context.Context) ([]tools.TeamInfo, error) {
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

// Ensure graphqlClientAdapter implements tools.GraphQLClient
var _ tools.GraphQLClient = (*graphqlClientAdapter)(nil)

// buildConsoleURLs builds the console base URL and URL patterns for a tenant.
func buildConsoleURLs(tenantName string) (string, map[string]string) {
	baseURL := fmt.Sprintf("https://console.%s.cloud.nais.io", tenantName)

	patterns := map[string]string{
		"team":        baseURL + "/team/{team}",
		"app":         baseURL + "/team/{team}/{env}/app/{app}",
		"job":         baseURL + "/team/{team}/{env}/job/{job}",
		"deployment":  baseURL + "/team/{team}/deployments",
		"cost":        baseURL + "/team/{team}/cost",
		"utilization": baseURL + "/team/{team}/utilization",
		"secrets":     baseURL + "/team/{team}/{env}/secret/{secret}",
		"postgres":    baseURL + "/team/{team}/{env}/postgres/{instance}",
		"bucket":      baseURL + "/team/{team}/{env}/bucket/{bucket}",
		"redis":       baseURL + "/team/{team}/{env}/redis/{instance}",
		"opensearch":  baseURL + "/team/{team}/{env}/opensearch/{instance}",
		"kafka":       baseURL + "/team/{team}/{env}/kafka/{topic}",
	}

	return baseURL, patterns
}
