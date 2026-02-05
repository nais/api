// Package tools provides MCP tool definitions and execution.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/vektah/gqlparser/v2/ast"
)

// Registry holds all tool definitions with their handlers.
// It provides a single source of truth for tools used by both
// the MCP server (CLI) and the agent executor.
type Registry struct {
	tools        map[string]RegisteredTool
	toolOrder    []string // Maintains registration order for consistent iteration
	schemaTools  *SchemaTools
	graphqlTools *GraphQLTools
	logger       *slog.Logger
	mu           sync.RWMutex
}

// RegisteredTool combines an mcp-go Tool definition with its handler.
type RegisteredTool struct {
	Tool    mcp.Tool
	Handler ToolHandler
}

// ToolHandler is the signature for tool handlers.
type ToolHandler func(ctx context.Context, args map[string]any) (any, error)

// RegistryConfig holds configuration for creating a Registry.
type RegistryConfig struct {
	// Client is the GraphQL client for executing queries.
	Client GraphQLClient

	// Schema is the parsed GraphQL schema.
	Schema *ast.Schema

	// ConsoleBaseURL is the base URL for the Nais console.
	ConsoleBaseURL string

	// ConsoleURLPatterns are URL patterns for console pages.
	ConsoleURLPatterns map[string]string

	// Logger is the logger for tool operations.
	Logger *slog.Logger
}

// NewRegistry creates a new tool registry with all tools registered.
func NewRegistry(cfg RegistryConfig) *Registry {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	r := &Registry{
		tools:        make(map[string]RegisteredTool),
		toolOrder:    make([]string, 0),
		schemaTools:  NewSchemaTools(cfg.Schema),
		graphqlTools: NewGraphQLTools(cfg.Client, cfg.Schema, cfg.ConsoleBaseURL, cfg.ConsoleURLPatterns),
		logger:       cfg.Logger,
	}

	r.registerAllTools()
	return r
}

// register adds a tool to the registry.
func (r *Registry) register(tool mcp.Tool, handler ToolHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Name] = RegisteredTool{
		Tool:    tool,
		Handler: handler,
	}
	r.toolOrder = append(r.toolOrder, tool.Name)
}

// registerAllTools registers all available tools.
func (r *Registry) registerAllTools() {
	// Schema exploration tools
	r.registerSchemaListTypes()
	r.registerSchemaGetType()
	r.registerSchemaListQueries()
	r.registerSchemaListMutations()
	r.registerSchemaGetField()
	r.registerSchemaGetEnum()
	r.registerSchemaSearch()
	r.registerSchemaGetImplementors()
	r.registerSchemaGetUnionTypes()

	// GraphQL execution tools
	r.registerGetNaisContext()
	r.registerExecuteGraphQL()
	r.registerValidateGraphQL()
}

// Schema tool registrations

func (r *Registry) registerSchemaListTypes() {
	r.register(
		mcp.NewTool("schema_list_types",
			mcp.WithDescription("List all types in the Nais GraphQL API schema, grouped by kind. Use this to explore available data types before querying specific type details. Useful for understanding the API structure."),
			mcp.WithInputSchema[SchemaListTypesInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaListTypes,
	)
}

func (r *Registry) registerSchemaGetType() {
	r.register(
		mcp.NewTool("schema_get_type",
			mcp.WithDescription("Get complete details about a GraphQL type: fields with their types, interfaces it implements, types that implement it (for interfaces), enum values, or union member types. Use this to understand the shape of data returned by queries."),
			mcp.WithInputSchema[SchemaGetTypeInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaGetType,
	)
}

func (r *Registry) registerSchemaListQueries() {
	r.register(
		mcp.NewTool("schema_list_queries",
			mcp.WithDescription("List all available GraphQL query operations with their return types and number of arguments. These are the entry points for reading data from the Nais API."),
			mcp.WithInputSchema[SchemaListQueriesInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaListQueries,
	)
}

func (r *Registry) registerSchemaListMutations() {
	r.register(
		mcp.NewTool("schema_list_mutations",
			mcp.WithDescription("List all available GraphQL mutation operations with their return types and number of arguments. Mutations are used to modify data (note: the MCP server currently only exposes read operations)."),
			mcp.WithInputSchema[SchemaListMutationsInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaListMutations,
	)
}

func (r *Registry) registerSchemaGetField() {
	r.register(
		mcp.NewTool("schema_get_field",
			mcp.WithDescription("Get detailed information about a specific field including its arguments with types and defaults, return type, description, and deprecation status. Use 'Query' as the type to inspect query operations, or 'Mutation' for mutations."),
			mcp.WithInputSchema[SchemaGetFieldInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaGetField,
	)
}

func (r *Registry) registerSchemaGetEnum() {
	r.register(
		mcp.NewTool("schema_get_enum",
			mcp.WithDescription("Get all possible values for an enum type with their descriptions and deprecation status. Use this to understand valid values for enum fields (e.g., ApplicationState, DeploymentState)."),
			mcp.WithInputSchema[SchemaGetEnumInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaGetEnum,
	)
}

func (r *Registry) registerSchemaSearch() {
	r.register(
		mcp.NewTool("schema_search",
			mcp.WithDescription("Search across all schema types, fields, and enum values by name or description. Returns up to 50 matches. Use this to discover relevant types when you're not sure of exact names."),
			mcp.WithInputSchema[SchemaSearchInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaSearch,
	)
}

func (r *Registry) registerSchemaGetImplementors() {
	r.register(
		mcp.NewTool("schema_get_implementors",
			mcp.WithDescription("Get all concrete types that implement a GraphQL interface. Use this to find all possible types when a query returns an interface type."),
			mcp.WithInputSchema[SchemaGetImplementorsInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaGetImplementors,
	)
}

func (r *Registry) registerSchemaGetUnionTypes() {
	r.register(
		mcp.NewTool("schema_get_union_types",
			mcp.WithDescription("Get all member types of a GraphQL union. Use this to understand what concrete types can be returned when a query returns a union type."),
			mcp.WithInputSchema[SchemaGetUnionTypesInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeSchemaGetUnionTypes,
	)
}

// GraphQL tool registrations

func (r *Registry) registerGetNaisContext() {
	r.register(
		mcp.NewTool("get_nais_context",
			mcp.WithDescription("Get the current Nais context including authenticated user, their teams, and console URL. Call this first to understand what the user has access to and to get the correct console URL for links."),
			mcp.WithInputSchema[GetNaisContextInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeGetNaisContext,
	)
}

func (r *Registry) registerExecuteGraphQL() {
	r.register(
		mcp.NewTool("execute_graphql",
			mcp.WithDescription("Execute a GraphQL query against the Nais API.\n\nIMPORTANT: Before using this tool, use the schema exploration tools (schema_list_queries, schema_get_type, schema_get_field) to understand the available types and fields.\n\nThis tool only supports queries (read operations). Mutations are not allowed.\n\n"+NaisAPIGuidance),
			mcp.WithInputSchema[ExecuteGraphQLInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithOpenWorldHintAnnotation(true),
		),
		r.executeGraphQL,
	)
}

func (r *Registry) registerValidateGraphQL() {
	r.register(
		mcp.NewTool("validate_graphql",
			mcp.WithDescription("Validate a GraphQL query against the schema without executing it. Use this to check if your query is valid before executing."),
			mcp.WithInputSchema[ValidateGraphQLInput](),
			mcp.WithReadOnlyHintAnnotation(true),
			mcp.WithIdempotentHintAnnotation(true),
		),
		r.executeValidateGraphQL,
	)
}

// Tool handlers - Schema tools

func (r *Registry) executeSchemaListTypes(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaListTypesInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.ListTypes(ctx, input)
}

func (r *Registry) executeSchemaGetType(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaGetTypeInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.GetType(ctx, input)
}

func (r *Registry) executeSchemaListQueries(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaListQueriesInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.ListQueries(ctx, input)
}

func (r *Registry) executeSchemaListMutations(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaListMutationsInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.ListMutations(ctx, input)
}

func (r *Registry) executeSchemaGetField(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaGetFieldInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.GetField(ctx, input)
}

func (r *Registry) executeSchemaGetEnum(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaGetEnumInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.GetEnum(ctx, input)
}

func (r *Registry) executeSchemaSearch(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaSearchInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.Search(ctx, input)
}

func (r *Registry) executeSchemaGetImplementors(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaGetImplementorsInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.GetImplementors(ctx, input)
}

func (r *Registry) executeSchemaGetUnionTypes(ctx context.Context, args map[string]any) (any, error) {
	var input SchemaGetUnionTypesInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.schemaTools.GetUnionTypes(ctx, input)
}

// Tool handlers - GraphQL tools

func (r *Registry) executeGetNaisContext(ctx context.Context, args map[string]any) (any, error) {
	var input GetNaisContextInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.graphqlTools.GetNaisContext(ctx, input)
}

func (r *Registry) executeGraphQL(ctx context.Context, args map[string]any) (any, error) {
	var input ExecuteGraphQLInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.graphqlTools.ExecuteGraphQL(ctx, input)
}

func (r *Registry) executeValidateGraphQL(ctx context.Context, args map[string]any) (any, error) {
	var input ValidateGraphQLInput
	if err := bindArgs(args, &input); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}
	return r.graphqlTools.ValidateGraphQL(ctx, input)
}

// Public API methods

// Execute runs a tool by name with the given arguments.
// This is the main method for the agent to call tools directly.
func (r *Registry) Execute(ctx context.Context, name string, args map[string]any) (any, error) {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}

	r.logger.Debug("executing tool", "name", name)
	return tool.Handler(ctx, args)
}

// GetMCPTools returns all tools in mcp-go format for the MCP server.
func (r *Registry) GetMCPTools() []mcp.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]mcp.Tool, 0, len(r.toolOrder))
	for _, name := range r.toolOrder {
		if tool, ok := r.tools[name]; ok {
			result = append(result, tool.Tool)
		}
	}
	return result
}

// GetMCPServerTools returns tools as server.ServerTool for adding to an MCP server.
func (r *Registry) GetMCPServerTools() []server.ServerTool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]server.ServerTool, 0, len(r.toolOrder))
	for _, name := range r.toolOrder {
		if tool, ok := r.tools[name]; ok {
			// Capture loop variable for closure
			handler := tool.Handler
			result = append(result, server.ServerTool{
				Tool:    tool.Tool,
				Handler: r.wrapHandler(handler),
			})
		}
	}
	return result
}

// CreateMCPHandler creates an MCP server handler function for a tool.
func (r *Registry) CreateMCPHandler(name string) server.ToolHandlerFunc {
	r.mu.RLock()
	tool, ok := r.tools[name]
	r.mu.RUnlock()

	if !ok {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return mcp.NewToolResultError(fmt.Sprintf("unknown tool: %s", name)), nil
		}
	}

	return r.wrapHandler(tool.Handler)
}

// wrapHandler wraps a ToolHandler to produce an MCP server handler.
func (r *Registry) wrapHandler(handler ToolHandler) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		if args == nil {
			args = make(map[string]any)
		}

		result, err := handler(ctx, args)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Marshal result to JSON
		jsonData, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to marshal result: %v", err)), nil
		}

		return mcp.NewToolResultText(string(jsonData)), nil
	}
}

// GetToolDefinitions returns tool definitions suitable for LLM consumption.
// This includes the full JSON schema for each tool's input.
func (r *Registry) GetToolDefinitions() []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ToolDefinition, 0, len(r.toolOrder))
	for _, name := range r.toolOrder {
		if tool, ok := r.tools[name]; ok {
			def := ToolDefinition{
				Name:        tool.Tool.Name,
				Description: tool.Tool.Description,
			}

			// Include the full input schema
			if tool.Tool.RawInputSchema != nil {
				// Parse raw schema to map for easier consumption
				var schema map[string]any
				if err := json.Unmarshal(tool.Tool.RawInputSchema, &schema); err == nil {
					def.InputSchema = schema
				} else {
					def.InputSchema = tool.Tool.RawInputSchema
				}
			} else if tool.Tool.InputSchema.Type != "" {
				def.InputSchema = tool.Tool.InputSchema
			}

			result = append(result, def)
		}
	}
	return result
}

// GetTool returns a specific tool by name.
func (r *Registry) GetTool(name string) (mcp.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if tool, ok := r.tools[name]; ok {
		return tool.Tool, true
	}
	return mcp.Tool{}, false
}

// ListToolNames returns the names of all registered tools.
func (r *Registry) ListToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]string, len(r.toolOrder))
	copy(result, r.toolOrder)
	return result
}

// bindArgs converts a map to a struct using JSON marshaling.
func bindArgs(args map[string]any, target any) error {
	if args == nil {
		args = make(map[string]any)
	}
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
