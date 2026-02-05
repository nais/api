package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// mockGraphQLClientForRegistry implements GraphQLClient for testing
type mockGraphQLClientForRegistry struct {
	executeGraphQLFn func(ctx context.Context, query string, variables map[string]any) (map[string]any, error)
	getCurrentUserFn func(ctx context.Context) (*UserInfo, error)
	getUserTeamsFn   func(ctx context.Context) ([]TeamInfo, error)
}

func (m *mockGraphQLClientForRegistry) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	if m.executeGraphQLFn != nil {
		return m.executeGraphQLFn(ctx, query, variables)
	}
	return map[string]any{}, nil
}

func (m *mockGraphQLClientForRegistry) GetCurrentUser(ctx context.Context) (*UserInfo, error) {
	if m.getCurrentUserFn != nil {
		return m.getCurrentUserFn(ctx)
	}
	return &UserInfo{Name: "test-user"}, nil
}

func (m *mockGraphQLClientForRegistry) GetUserTeams(ctx context.Context) ([]TeamInfo, error) {
	if m.getUserTeamsFn != nil {
		return m.getUserTeamsFn(ctx)
	}
	return []TeamInfo{{Slug: "test-team", Role: "owner"}}, nil
}

func createTestRegistry(t *testing.T) *Registry {
	t.Helper()

	schemaSource := `
		type Query {
			team(slug: String!): Team
			me: User
		}

		type User {
			name: String!
			teams: [Team!]!
		}

		type Team {
			slug: String!
			name: String!
			applications: [Application!]!
		}

		type Application {
			name: String!
			state: ApplicationState!
		}

		enum ApplicationState {
			RUNNING
			NOT_RUNNING
			UNKNOWN
		}
	`

	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "test.graphql", Input: schemaSource})
	if err != nil {
		t.Fatalf("failed to parse test schema: %v", err)
	}

	client := &mockGraphQLClientForRegistry{}

	return NewRegistry(RegistryConfig{
		Client:             client,
		Schema:             schema,
		ConsoleBaseURL:     "https://console.example.com",
		ConsoleURLPatterns: map[string]string{"team": "/team/{team}"},
	})
}

func TestRegistry_ListToolNames(t *testing.T) {
	registry := createTestRegistry(t)

	names := registry.ListToolNames()

	expectedTools := []string{
		"schema_list_types",
		"schema_get_type",
		"schema_list_queries",
		"schema_list_mutations",
		"schema_get_field",
		"schema_get_enum",
		"schema_search",
		"schema_get_implementors",
		"schema_get_union_types",
		"get_nais_context",
		"execute_graphql",
		"validate_graphql",
	}

	if len(names) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(names))
	}

	// Check that all expected tools are registered
	toolSet := make(map[string]bool)
	for _, name := range names {
		toolSet[name] = true
	}

	for _, expected := range expectedTools {
		if !toolSet[expected] {
			t.Errorf("expected tool %q to be registered", expected)
		}
	}
}

func TestRegistry_GetTool(t *testing.T) {
	registry := createTestRegistry(t)

	t.Run("existing tool", func(t *testing.T) {
		tool, ok := registry.GetTool("schema_get_type")
		if !ok {
			t.Fatal("expected tool to exist")
		}

		if tool.Name != "schema_get_type" {
			t.Errorf("expected tool name 'schema_get_type', got %q", tool.Name)
		}

		if tool.Description == "" {
			t.Error("expected tool to have a description")
		}
	})

	t.Run("non-existing tool", func(t *testing.T) {
		_, ok := registry.GetTool("non_existing_tool")
		if ok {
			t.Error("expected tool to not exist")
		}
	})
}

func TestRegistry_GetMCPTools(t *testing.T) {
	registry := createTestRegistry(t)

	tools := registry.GetMCPTools()

	if len(tools) == 0 {
		t.Fatal("expected at least one tool")
	}

	// Verify tools have the expected structure
	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool name should not be empty")
		}
		if tool.Description == "" {
			t.Errorf("tool %q should have a description", tool.Name)
		}
	}
}

func TestRegistry_GetToolDefinitions(t *testing.T) {
	registry := createTestRegistry(t)

	definitions := registry.GetToolDefinitions()

	if len(definitions) == 0 {
		t.Fatal("expected at least one tool definition")
	}

	// Find a tool with input schema and verify it has proper schema
	for _, def := range definitions {
		if def.Name == "schema_get_type" {
			if def.InputSchema == nil {
				t.Error("expected schema_get_type to have input schema")
				continue
			}

			// InputSchema should be a map with JSON schema structure
			schemaMap, ok := def.InputSchema.(map[string]any)
			if !ok {
				t.Errorf("expected input schema to be a map, got %T", def.InputSchema)
				continue
			}

			// Should have properties
			if _, hasProps := schemaMap["properties"]; !hasProps {
				t.Error("expected input schema to have 'properties' field")
			}

			// Check for the 'name' property
			if props, ok := schemaMap["properties"].(map[string]any); ok {
				if _, hasName := props["name"]; !hasName {
					t.Error("expected 'name' property in schema_get_type input schema")
				}
			}

			// Should have required fields
			if _, hasRequired := schemaMap["required"]; !hasRequired {
				t.Error("expected input schema to have 'required' field")
			}
		}
	}
}

func TestRegistry_GetToolDefinitions_HasInputSchema(t *testing.T) {
	registry := createTestRegistry(t)

	definitions := registry.GetToolDefinitions()

	// Tools that should have required parameters
	toolsWithRequiredParams := map[string][]string{
		"schema_get_type":         {"name"},
		"schema_get_field":        {"type", "field"},
		"schema_get_enum":         {"name"},
		"schema_search":           {"query"},
		"schema_get_implementors": {"interface"},
		"schema_get_union_types":  {"union"},
		"execute_graphql":         {"query"},
		"validate_graphql":        {"query"},
	}

	for _, def := range definitions {
		expectedRequired, hasExpected := toolsWithRequiredParams[def.Name]
		if !hasExpected {
			continue
		}

		if def.InputSchema == nil {
			t.Errorf("tool %q should have input schema", def.Name)
			continue
		}

		schemaMap, ok := def.InputSchema.(map[string]any)
		if !ok {
			t.Errorf("tool %q input schema should be a map", def.Name)
			continue
		}

		required, ok := schemaMap["required"].([]any)
		if !ok {
			// Try []string
			requiredStr, ok := schemaMap["required"].([]string)
			if !ok {
				t.Errorf("tool %q should have 'required' array in schema", def.Name)
				continue
			}
			// Convert to []any for comparison
			required = make([]any, len(requiredStr))
			for i, s := range requiredStr {
				required[i] = s
			}
		}

		// Check that all expected required fields are present
		for _, expectedField := range expectedRequired {
			found := false
			for _, r := range required {
				if r == expectedField {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("tool %q should have %q as required field", def.Name, expectedField)
			}
		}
	}
}

func TestRegistry_Execute(t *testing.T) {
	registry := createTestRegistry(t)

	t.Run("schema_list_types", func(t *testing.T) {
		result, err := registry.Execute(context.Background(), "schema_list_types", map[string]any{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output, ok := result.(SchemaListTypesOutput)
		if !ok {
			t.Fatalf("expected SchemaListTypesOutput, got %T", result)
		}

		// Should have at least some types
		if len(output.Objects) == 0 {
			t.Error("expected at least one object type")
		}
	})

	t.Run("schema_get_type", func(t *testing.T) {
		result, err := registry.Execute(context.Background(), "schema_get_type", map[string]any{
			"name": "Team",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		output, ok := result.(SchemaGetTypeOutput)
		if !ok {
			t.Fatalf("expected SchemaGetTypeOutput, got %T", result)
		}

		if output.Name != "Team" {
			t.Errorf("expected type name 'Team', got %q", output.Name)
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		_, err := registry.Execute(context.Background(), "unknown_tool", map[string]any{})
		if err == nil {
			t.Error("expected error for unknown tool")
		}
	})
}

func TestRegistry_GetMCPServerTools(t *testing.T) {
	registry := createTestRegistry(t)

	serverTools := registry.GetMCPServerTools()

	if len(serverTools) == 0 {
		t.Fatal("expected at least one server tool")
	}

	// Each server tool should have both a Tool and a Handler
	for _, st := range serverTools {
		if st.Tool.Name == "" {
			t.Error("server tool should have a name")
		}
		if st.Handler == nil {
			t.Errorf("server tool %q should have a handler", st.Tool.Name)
		}
	}
}

func TestRegistry_CreateMCPHandler(t *testing.T) {
	registry := createTestRegistry(t)

	t.Run("existing tool", func(t *testing.T) {
		handler := registry.CreateMCPHandler("schema_list_types")
		if handler == nil {
			t.Fatal("expected handler to be created")
		}
	})

	t.Run("unknown tool", func(t *testing.T) {
		handler := registry.CreateMCPHandler("unknown_tool")
		if handler == nil {
			t.Fatal("expected handler to be created (for error response)")
		}
	})
}

func TestRegistry_ToolAnnotations(t *testing.T) {
	registry := createTestRegistry(t)

	tools := registry.GetMCPTools()

	// Find a read-only tool and verify its annotations
	for _, tool := range tools {
		if tool.Name == "schema_get_type" {
			// Should be marked as read-only
			if tool.Annotations.ReadOnlyHint == nil || !*tool.Annotations.ReadOnlyHint {
				t.Error("schema_get_type should be marked as read-only")
			}
			// Should be marked as idempotent
			if tool.Annotations.IdempotentHint == nil || !*tool.Annotations.IdempotentHint {
				t.Error("schema_get_type should be marked as idempotent")
			}
			break
		}
	}

	// execute_graphql should have OpenWorldHint
	for _, tool := range tools {
		if tool.Name == "execute_graphql" {
			if tool.Annotations.OpenWorldHint == nil || !*tool.Annotations.OpenWorldHint {
				t.Error("execute_graphql should be marked with OpenWorldHint")
			}
			break
		}
	}
}

func TestRegistry_SchemaGenerationFormat(t *testing.T) {
	registry := createTestRegistry(t)

	definitions := registry.GetToolDefinitions()

	// Find schema_get_type and verify the schema can be serialized to JSON
	for _, def := range definitions {
		if def.Name == "schema_get_type" {
			schemaJSON, err := json.Marshal(def.InputSchema)
			if err != nil {
				t.Fatalf("failed to marshal input schema to JSON: %v", err)
			}

			// Parse it back and verify structure
			var schema map[string]any
			if err := json.Unmarshal(schemaJSON, &schema); err != nil {
				t.Fatalf("failed to unmarshal schema JSON: %v", err)
			}

			// Should be an object type
			if schemaType, ok := schema["type"].(string); ok {
				if schemaType != "object" {
					t.Errorf("expected schema type 'object', got %q", schemaType)
				}
			}

			t.Logf("schema_get_type input schema: %s", string(schemaJSON))
			break
		}
	}
}

func TestToolDefinition_GetParameters(t *testing.T) {
	registry := createTestRegistry(t)
	definitions := registry.GetToolDefinitions()

	t.Run("schema_get_type has required name parameter", func(t *testing.T) {
		var toolDef ToolDefinition
		for _, def := range definitions {
			if def.Name == "schema_get_type" {
				toolDef = def
				break
			}
		}

		if toolDef.Name == "" {
			t.Fatal("schema_get_type tool not found")
		}

		params := toolDef.GetParameters()
		if len(params) == 0 {
			t.Fatal("expected at least one parameter")
		}

		// Find the 'name' parameter
		var nameParam *ParameterInfo
		for i := range params {
			if params[i].Name == "name" {
				nameParam = &params[i]
				break
			}
		}

		if nameParam == nil {
			t.Fatal("expected 'name' parameter")
		}

		if nameParam.Type != "string" {
			t.Errorf("expected type 'string', got %q", nameParam.Type)
		}
		if !nameParam.Required {
			t.Error("expected 'name' parameter to be required")
		}
		if nameParam.Description == "" {
			t.Error("expected 'name' parameter to have a description")
		}
	})

	t.Run("schema_list_types has optional parameters", func(t *testing.T) {
		var toolDef ToolDefinition
		for _, def := range definitions {
			if def.Name == "schema_list_types" {
				toolDef = def
				break
			}
		}

		if toolDef.Name == "" {
			t.Fatal("schema_list_types tool not found")
		}

		params := toolDef.GetParameters()
		if len(params) != 2 {
			t.Fatalf("expected 2 parameters, got %d", len(params))
		}

		// Both 'kind' and 'search' should be optional
		for _, param := range params {
			if param.Required {
				t.Errorf("expected parameter %q to be optional", param.Name)
			}
		}
	})

	t.Run("get_nais_context has no parameters", func(t *testing.T) {
		var toolDef ToolDefinition
		for _, def := range definitions {
			if def.Name == "get_nais_context" {
				toolDef = def
				break
			}
		}

		if toolDef.Name == "" {
			t.Fatal("get_nais_context tool not found")
		}

		params := toolDef.GetParameters()
		if len(params) != 0 {
			t.Errorf("expected 0 parameters, got %d", len(params))
		}
	})

	t.Run("execute_graphql has required query and optional variables", func(t *testing.T) {
		var toolDef ToolDefinition
		for _, def := range definitions {
			if def.Name == "execute_graphql" {
				toolDef = def
				break
			}
		}

		if toolDef.Name == "" {
			t.Fatal("execute_graphql tool not found")
		}

		params := toolDef.GetParameters()
		if len(params) != 2 {
			t.Fatalf("expected 2 parameters, got %d", len(params))
		}

		// Check parameters
		paramMap := make(map[string]ParameterInfo)
		for _, p := range params {
			paramMap[p.Name] = p
		}

		queryParam, ok := paramMap["query"]
		if !ok {
			t.Fatal("expected 'query' parameter")
		}
		if !queryParam.Required {
			t.Error("expected 'query' parameter to be required")
		}

		variablesParam, ok := paramMap["variables"]
		if !ok {
			t.Fatal("expected 'variables' parameter")
		}
		if variablesParam.Required {
			t.Error("expected 'variables' parameter to be optional")
		}
	})
}

func TestToolDefinition_GetParameters_NilSchema(t *testing.T) {
	def := ToolDefinition{
		Name:        "test",
		Description: "test tool",
		InputSchema: nil,
	}

	params := def.GetParameters()
	if params != nil {
		t.Errorf("expected nil parameters for nil schema, got %v", params)
	}
}
