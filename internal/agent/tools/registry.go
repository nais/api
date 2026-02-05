// Package tools provides tool definitions and execution for the agent.
// This is a simplified implementation for direct LLM integration without MCP protocol overhead.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/vektah/gqlparser/v2/ast"
)

// Tool represents a tool that can be called by the LLM.
type Tool struct {
	Name        string
	Description string
	Parameters  []Parameter
	Handler     Handler
}

// Parameter describes a tool parameter.
type Parameter struct {
	Name        string
	Type        string
	Description string
	Required    bool
}

// Handler is the function signature for tool handlers.
type Handler func(ctx context.Context, args map[string]any) (any, error)

// Registry holds all available tools.
type Registry struct {
	tools        map[string]Tool
	toolOrder    []string // Maintains registration order
	schemaTools  *SchemaTools
	graphqlTools *GraphQLTools
	logger       *slog.Logger
	mu           sync.RWMutex
}

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
		tools:        make(map[string]Tool),
		toolOrder:    make([]string, 0),
		schemaTools:  NewSchemaTools(cfg.Schema),
		graphqlTools: NewGraphQLTools(cfg.Client, cfg.Schema, cfg.ConsoleBaseURL, cfg.ConsoleURLPatterns),
		logger:       cfg.Logger,
	}

	r.registerAllTools()
	return r
}

// register adds a tool to the registry.
func (r *Registry) register(tool Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[tool.Name] = tool
	r.toolOrder = append(r.toolOrder, tool.Name)
}

// registerAllTools registers all available tools.
func (r *Registry) registerAllTools() {
	// Schema exploration tools
	r.register(Tool{
		Name:        "schema_list_types",
		Description: "List all types in the Nais GraphQL API schema, grouped by kind. Use this to explore available data types before querying specific type details.",
		Parameters: []Parameter{
			{Name: "kind", Type: "string", Description: "Filter by kind: 'OBJECT', 'INTERFACE', 'ENUM', 'UNION', 'INPUT_OBJECT', 'SCALAR', or 'all' (default: 'all')", Required: false},
			{Name: "search", Type: "string", Description: "Filter type names containing this string (case-insensitive)", Required: false},
		},
		Handler: r.executeSchemaListTypes,
	})

	r.register(Tool{
		Name:        "schema_get_type",
		Description: "Get complete details about a GraphQL type: fields with their types, interfaces it implements, types that implement it (for interfaces), enum values, or union member types.",
		Parameters: []Parameter{
			{Name: "name", Type: "string", Description: "The exact type name (e.g., 'Application', 'Team', 'DeploymentState')", Required: true},
		},
		Handler: r.executeSchemaGetType,
	})

	r.register(Tool{
		Name:        "schema_list_queries",
		Description: "List all available GraphQL query operations with their return types and number of arguments. These are the entry points for reading data from the Nais API.",
		Parameters: []Parameter{
			{Name: "search", Type: "string", Description: "Filter query names or descriptions containing this string (case-insensitive)", Required: false},
		},
		Handler: r.executeSchemaListQueries,
	})

	r.register(Tool{
		Name:        "schema_list_mutations",
		Description: "List all available GraphQL mutation operations with their return types and number of arguments. Mutations are used to modify data (note: the agent currently only exposes read operations).",
		Parameters: []Parameter{
			{Name: "search", Type: "string", Description: "Filter mutation names or descriptions containing this string (case-insensitive)", Required: false},
		},
		Handler: r.executeSchemaListMutations,
	})

	r.register(Tool{
		Name:        "schema_get_field",
		Description: "Get detailed information about a specific field including its arguments with types and defaults, return type, description, and deprecation status. Use 'Query' as the type to inspect query operations, or 'Mutation' for mutations.",
		Parameters: []Parameter{
			{Name: "type", Type: "string", Description: "The type name containing the field (use 'Query' for root queries, 'Mutation' for root mutations, or any object type name)", Required: true},
			{Name: "field", Type: "string", Description: "The field name to inspect", Required: true},
		},
		Handler: r.executeSchemaGetField,
	})

	r.register(Tool{
		Name:        "schema_get_enum",
		Description: "Get all possible values for an enum type with their descriptions and deprecation status. Use this to understand valid values for enum fields (e.g., ApplicationState, DeploymentState).",
		Parameters: []Parameter{
			{Name: "name", Type: "string", Description: "The enum type name (e.g., 'ApplicationState', 'TeamRole')", Required: true},
		},
		Handler: r.executeSchemaGetEnum,
	})

	r.register(Tool{
		Name:        "schema_search",
		Description: "Search across all schema types, fields, and enum values by name or description. Returns up to 50 matches. Use this to discover relevant types when you're not sure of exact names.",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Description: "Search term to match against names and descriptions (case-insensitive)", Required: true},
		},
		Handler: r.executeSchemaSearch,
	})

	r.register(Tool{
		Name:        "schema_get_implementors",
		Description: "Get all concrete types that implement a GraphQL interface. Use this to find all possible types when a query returns an interface type.",
		Parameters: []Parameter{
			{Name: "interface", Type: "string", Description: "The interface name (e.g., 'Workload', 'Issue')", Required: true},
		},
		Handler: r.executeSchemaGetImplementors,
	})

	r.register(Tool{
		Name:        "schema_get_union_types",
		Description: "Get all member types of a GraphQL union. Use this to understand what concrete types can be returned when a query returns a union type.",
		Parameters: []Parameter{
			{Name: "union", Type: "string", Description: "The union type name", Required: true},
		},
		Handler: r.executeSchemaGetUnionTypes,
	})

	// GraphQL execution tools
	r.register(Tool{
		Name:        "get_nais_context",
		Description: "Get the current Nais context including authenticated user, their teams, and console URL. Call this first to understand what the user has access to and to get the correct console URL for links.",
		Parameters:  []Parameter{},
		Handler:     r.executeGetNaisContext,
	})

	r.register(Tool{
		Name:        "execute_graphql",
		Description: "Execute a GraphQL query against the Nais API.\n\nIMPORTANT: Before using this tool, use the schema exploration tools (schema_list_queries, schema_get_type, schema_get_field) to understand the available types and fields.\n\nThis tool only supports queries (read operations). Mutations are not allowed.\n\n" + NaisAPIGuidance,
		Parameters: []Parameter{
			{Name: "query", Type: "string", Description: "The GraphQL query to execute. Must be a query operation (not mutation or subscription).", Required: true},
			{Name: "variables", Type: "string", Description: "JSON object containing variables for the query. Example: {\"slug\": \"my-team\", \"first\": 10}", Required: false},
		},
		Handler: r.executeGraphQL,
	})

	r.register(Tool{
		Name:        "validate_graphql",
		Description: "Validate a GraphQL query against the schema without executing it. Use this to check if your query is valid before executing.",
		Parameters: []Parameter{
			{Name: "query", Type: "string", Description: "The GraphQL query to validate.", Required: true},
		},
		Handler: r.executeValidateGraphQL,
	})
}

// Execute runs a tool by name with the given arguments.
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

// ListTools returns all registered tools.
func (r *Registry) ListTools() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.toolOrder))
	for _, name := range r.toolOrder {
		if tool, ok := r.tools[name]; ok {
			result = append(result, tool)
		}
	}
	return result
}

// Tool handlers - Schema tools

func (r *Registry) executeSchemaListTypes(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaListTypesInput{
		Kind:   getString(args, "kind"),
		Search: getString(args, "search"),
	}
	return r.schemaTools.ListTypes(ctx, input)
}

func (r *Registry) executeSchemaGetType(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaGetTypeInput{
		Name: getString(args, "name"),
	}
	return r.schemaTools.GetType(ctx, input)
}

func (r *Registry) executeSchemaListQueries(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaListQueriesInput{
		Search: getString(args, "search"),
	}
	return r.schemaTools.ListQueries(ctx, input)
}

func (r *Registry) executeSchemaListMutations(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaListMutationsInput{
		Search: getString(args, "search"),
	}
	return r.schemaTools.ListMutations(ctx, input)
}

func (r *Registry) executeSchemaGetField(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaGetFieldInput{
		Type:  getString(args, "type"),
		Field: getString(args, "field"),
	}
	return r.schemaTools.GetField(ctx, input)
}

func (r *Registry) executeSchemaGetEnum(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaGetEnumInput{
		Name: getString(args, "name"),
	}
	return r.schemaTools.GetEnum(ctx, input)
}

func (r *Registry) executeSchemaSearch(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaSearchInput{
		Query: getString(args, "query"),
	}
	return r.schemaTools.Search(ctx, input)
}

func (r *Registry) executeSchemaGetImplementors(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaGetImplementorsInput{
		Interface: getString(args, "interface"),
	}
	return r.schemaTools.GetImplementors(ctx, input)
}

func (r *Registry) executeSchemaGetUnionTypes(ctx context.Context, args map[string]any) (any, error) {
	input := SchemaGetUnionTypesInput{
		Union: getString(args, "union"),
	}
	return r.schemaTools.GetUnionTypes(ctx, input)
}

// Tool handlers - GraphQL tools

func (r *Registry) executeGetNaisContext(ctx context.Context, args map[string]any) (any, error) {
	return r.graphqlTools.GetNaisContext(ctx)
}

func (r *Registry) executeGraphQL(ctx context.Context, args map[string]any) (any, error) {
	input := ExecuteGraphQLInput{
		Query:     getString(args, "query"),
		Variables: getString(args, "variables"),
	}
	return r.graphqlTools.ExecuteGraphQL(ctx, input)
}

func (r *Registry) executeValidateGraphQL(ctx context.Context, args map[string]any) (any, error) {
	input := ValidateGraphQLInput{
		Query: getString(args, "query"),
	}
	return r.graphqlTools.ValidateGraphQL(ctx, input)
}

// getString safely extracts a string from args map.
func getString(args map[string]any, key string) string {
	if v, ok := args[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		// Try JSON number or other types
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
	}
	return ""
}
