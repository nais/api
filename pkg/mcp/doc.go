// Package mcp provides Model Context Protocol (MCP) tools for interacting with the Nais GraphQL API.
//
// This package supports two use cases:
//
// 1. CLI/MCP Server: Exposes tools via the MCP protocol for use with AI assistants
// that support MCP (e.g., Claude Desktop, Cursor). Use [NewServer] to create an
// MCP server with stdio, HTTP, or SSE transports.
//
// 2. Hosted Agent: Provides direct tool execution without MCP protocol overhead.
// Use [NewExecutor] to create a tool executor that can be called directly from
// an AI agent orchestrator.
//
// # Tools
//
// The package provides two categories of tools:
//
// Schema exploration tools for discovering the GraphQL API structure:
//   - schema_list_types: List all types in the schema
//   - schema_get_type: Get details about a specific type
//   - schema_list_queries: List available query operations
//   - schema_list_mutations: List available mutation operations
//   - schema_get_field: Get details about a specific field
//   - schema_get_enum: Get enum values
//   - schema_search: Search the schema
//   - schema_get_implementors: Get types implementing an interface
//   - schema_get_union_types: Get member types of a union
//
// GraphQL execution tools for querying the Nais API:
//   - get_nais_context: Get current user, teams, and console URLs
//   - execute_graphql: Execute a GraphQL query
//   - validate_graphql: Validate a query without executing
//
// # Security
//
// The tools enforce security restrictions:
//   - Only query operations are allowed (no mutations via execute_graphql)
//   - Secret-related types and fields are blocked
//   - Query depth is limited to prevent abuse
//
// # Example (Hosted Agent)
//
//	executor := mcp.NewExecutor(
//	    mcp.WithGraphQLClient(client),
//	    mcp.WithSchemaProvider(schemaProvider),
//	    mcp.WithTenantName("nav"),
//	)
//
//	result, err := executor.ExecuteTool(ctx, "execute_graphql", map[string]any{
//	    "query": `query { team(slug: "my-team") { slug purpose } }`,
//	})
//
// # Example (MCP Server)
//
//	server, err := mcp.NewServer(
//	    mcp.WithTransport(mcp.TransportStdio),
//	    mcp.WithGraphQLClient(client),
//	    mcp.WithSchemaProvider(schemaProvider),
//	    mcp.WithTenantName("nav"),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	server.Serve(ctx)
package mcp
