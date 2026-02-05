package agent_test

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/nais/api/internal/agent"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
)

// This file provides examples of how to integrate the MCP tools
// with the agent orchestrator.

// Example_basicSetup demonstrates basic setup of the MCP integration.
func Example_basicSetup() {
	// Create the gqlgen handler (typically done during application startup)
	// This example shows the minimal setup - in practice, you'd use your
	// existing handler from graph.NewHandler()
	graphHandler := handler.New(gengql.NewExecutableSchema(gengql.Config{
		// Your resolver would go here
	}))

	// Create the MCP integration
	integration, err := agent.NewMCPIntegration(agent.MCPIntegrationConfig{
		Handler:    graphHandler,
		TenantName: "nav", // Used to build console URLs: console.<tenant>.cloud.nais.io
	})
	if err != nil {
		panic(err)
	}

	// The integration is now ready to use
	fmt.Printf("MCP integration created with %d tools\n", len(integration.ListTools()))
}

// Example_executeTool demonstrates how to execute an MCP tool.
func Example_executeTool() {
	// Assume we have an MCP integration (see Example_basicSetup)
	var integration *agent.MCPIntegration // = ...

	// Create a context with authenticated user
	// (this would typically come from the HTTP request)
	ctx := context.Background()
	// ctx = authz.ContextWithActor(ctx, user, roles)

	// Execute a schema exploration tool
	result, err := integration.ExecuteTool(ctx, "schema_list_types", map[string]any{
		"kind": "OBJECT",
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Schema types: %v\n", result)
}

// Example_executeGraphQL demonstrates how to execute a GraphQL query.
func Example_executeGraphQL() {
	// Assume we have an MCP integration
	var integration *agent.MCPIntegration // = ...

	ctx := context.Background()

	// Execute a GraphQL query
	result, err := integration.ExecuteTool(ctx, "execute_graphql", map[string]any{
		"query": `
			query($slug: Slug!) {
				team(slug: $slug) {
					slug
					purpose
					applications(first: 10) {
						nodes {
							name
							state
						}
					}
				}
			}
		`,
		"variables": `{"slug": "my-team"}`,
	})
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Query result: %v\n", result)
}

// Example_orchestratorIntegration demonstrates how to integrate MCP tools
// with the existing orchestrator for LLM-driven tool usage.
func Example_orchestratorIntegration() {
	// This example shows how you would modify the orchestrator to use
	// the MCP integration instead of direct HTTP calls.

	// 1. Create the MCP integration during application setup
	// integration, _ := agent.NewMCPIntegration(agent.MCPIntegrationConfig{...})

	// 2. In the orchestrator's executeTool method, dispatch to MCP tools:
	/*
		func (o *Orchestrator) executeTool(ctx context.Context, toolCall chat.ToolCall) (string, error) {
			// Use MCP integration for all supported tools
			result, err := o.mcpIntegration.ExecuteTool(ctx, toolCall.Name, toolCall.Arguments)
			if err != nil {
				return "", err
			}

			// Convert result to JSON string for LLM consumption
			resultJSON, err := json.Marshal(result)
			if err != nil {
				return "", fmt.Errorf("failed to marshal result: %w", err)
			}

			return string(resultJSON), nil
		}
	*/

	// 3. Update the tool definitions provided to the LLM:
	/*
		func getToolDefinitions(integration *agent.MCPIntegration) []chat.ToolDefinition {
			mcpTools := integration.GetToolDefinitions()
			result := make([]chat.ToolDefinition, len(mcpTools))
			for i, tool := range mcpTools {
				result[i] = chat.ToolDefinition{
					Name:        tool.Name,
					Description: tool.Description,
					// Add parameter schemas as needed
				}
			}
			return result
		}
	*/
}

// Example_directClientUsage demonstrates using the InternalClient directly.
func Example_directClientUsage() {
	// Create the internal client for direct GraphQL execution
	var graphHandler *handler.Server // = ... your gqlgen handler

	client := agent.NewInternalClient(agent.InternalClientConfig{
		Handler: graphHandler,
	})

	// Create context with authenticated user
	ctx := context.Background()
	// ctx = authz.ContextWithActor(ctx, user, roles)

	// Execute a GraphQL query directly
	result, err := client.ExecuteGraphQL(ctx, `
		query {
			me {
				... on User {
					email
					teams(first: 10) {
						nodes {
							team { slug }
						}
					}
				}
			}
		}
	`, nil)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Query result: %v\n", result)
}

// Example_withAuthentication demonstrates proper authentication setup.
func Example_withAuthentication() {
	// The MCP tools require an authenticated user in the context.
	// This is typically set up by authentication middleware.

	// Example of setting up context with a mock user (for testing):
	/*
		// Create or retrieve the user
		user, _ := user.GetByEmail(ctx, "test@example.com")

		// Get user's roles
		roles, _ := authz.ForUser(ctx, user.UUID)

		// Add user to context
		ctx = authz.ContextWithActor(ctx, user, roles)

		// Now you can use the MCP tools
		result, _ := integration.ExecuteTool(ctx, "get_nais_context", nil)
	*/

	// The get_nais_context tool returns:
	// - Current user info (name, email, isAdmin)
	// - User's teams with their roles
	// - Console URL patterns for building links
}

// Example_schemaExploration demonstrates schema exploration tools.
func Example_schemaExploration() {
	var integration *agent.MCPIntegration
	ctx := context.Background()

	// 1. List all types to understand the schema structure
	types, _ := integration.ExecuteTool(ctx, "schema_list_types", map[string]any{
		"kind": "OBJECT", // OBJECT, INTERFACE, ENUM, UNION, INPUT_OBJECT, SCALAR, or "all"
	})
	fmt.Printf("Types: %v\n", types)

	// 2. Get details about a specific type
	teamType, _ := integration.ExecuteTool(ctx, "schema_get_type", map[string]any{
		"name": "Team",
	})
	fmt.Printf("Team type: %v\n", teamType)

	// 3. List available queries
	queries, _ := integration.ExecuteTool(ctx, "schema_list_queries", map[string]any{
		"search": "team", // optional filter
	})
	fmt.Printf("Queries: %v\n", queries)

	// 4. Get field details including arguments
	field, _ := integration.ExecuteTool(ctx, "schema_get_field", map[string]any{
		"type":  "Query",
		"field": "team",
	})
	fmt.Printf("Field: %v\n", field)

	// 5. Search across the schema
	searchResults, _ := integration.ExecuteTool(ctx, "schema_search", map[string]any{
		"query": "application",
	})
	fmt.Printf("Search results: %v\n", searchResults)

	// 6. Get enum values
	enumValues, _ := integration.ExecuteTool(ctx, "schema_get_enum", map[string]any{
		"name": "ApplicationState",
	})
	fmt.Printf("Enum values: %v\n", enumValues)

	// 7. Get interface implementors
	implementors, _ := integration.ExecuteTool(ctx, "schema_get_implementors", map[string]any{
		"interface": "Workload",
	})
	fmt.Printf("Implementors: %v\n", implementors)

	// 8. Get union types
	unionTypes, _ := integration.ExecuteTool(ctx, "schema_get_union_types", map[string]any{
		"union": "Issue",
	})
	fmt.Printf("Union types: %v\n", unionTypes)
}

// Dummy to make the examples compile
var _ = authz.ContextWithActor
