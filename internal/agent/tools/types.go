// Package tools provides tool definitions and execution for the agent.
package tools

// =============================================================================
// Schema Tool Types
// =============================================================================

// SchemaListTypesInput is the input for the schema_list_types tool.
type SchemaListTypesInput struct {
	Kind   string `json:"kind,omitempty"`
	Search string `json:"search,omitempty"`
}

// SchemaListTypesOutput is the output for the schema_list_types tool.
type SchemaListTypesOutput struct {
	Objects      []string `json:"objects,omitempty"`
	Interfaces   []string `json:"interfaces,omitempty"`
	Enums        []string `json:"enums,omitempty"`
	Unions       []string `json:"unions,omitempty"`
	InputObjects []string `json:"input_objects,omitempty"`
	Scalars      []string `json:"scalars,omitempty"`
}

// SchemaGetTypeInput is the input for the schema_get_type tool.
type SchemaGetTypeInput struct {
	Name string `json:"name"`
}

// SchemaFieldInfo describes a field on a GraphQL type.
type SchemaFieldInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Deprecated  any    `json:"deprecated,omitempty"`
}

// SchemaEnumValue describes a value in a GraphQL enum.
type SchemaEnumValue struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Deprecated  any    `json:"deprecated,omitempty"`
}

// SchemaGetTypeOutput is the output for the schema_get_type tool.
type SchemaGetTypeOutput struct {
	Name          string            `json:"name"`
	Kind          string            `json:"kind"`
	Description   string            `json:"description,omitempty"`
	Implements    []string          `json:"implements,omitempty"`
	Fields        []SchemaFieldInfo `json:"fields,omitempty"`
	Values        []SchemaEnumValue `json:"values,omitempty"`
	Types         []string          `json:"types,omitempty"`
	ImplementedBy []string          `json:"implementedBy,omitempty"`
}

// SchemaListQueriesInput is the input for the schema_list_queries tool.
type SchemaListQueriesInput struct {
	Search string `json:"search,omitempty"`
}

// SchemaOperationInfo describes a GraphQL operation (query or mutation).
type SchemaOperationInfo struct {
	Name        string `json:"name"`
	ReturnType  string `json:"returnType"`
	Description string `json:"description,omitempty"`
	ArgCount    int    `json:"argCount"`
}

// SchemaListMutationsInput is the input for the schema_list_mutations tool.
type SchemaListMutationsInput struct {
	Search string `json:"search,omitempty"`
}

// SchemaGetFieldInput is the input for the schema_get_field tool.
type SchemaGetFieldInput struct {
	Type  string `json:"type"`
	Field string `json:"field"`
}

// SchemaArgumentInfo describes an argument on a GraphQL field.
type SchemaArgumentInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
}

// SchemaGetFieldOutput is the output for the schema_get_field tool.
type SchemaGetFieldOutput struct {
	Name        string               `json:"name"`
	Type        string               `json:"type"`
	Description string               `json:"description,omitempty"`
	Deprecated  any                  `json:"deprecated,omitempty"`
	Args        []SchemaArgumentInfo `json:"args,omitempty"`
}

// SchemaGetEnumInput is the input for the schema_get_enum tool.
type SchemaGetEnumInput struct {
	Name string `json:"name"`
}

// SchemaGetEnumOutput is the output for the schema_get_enum tool.
type SchemaGetEnumOutput struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Values      []SchemaEnumValue `json:"values"`
}

// SchemaSearchInput is the input for the schema_search tool.
type SchemaSearchInput struct {
	Query string `json:"query"`
}

// SchemaSearchResult is a single result from a schema search.
type SchemaSearchResult struct {
	Kind        string `json:"kind"`
	Name        string `json:"name"`
	Type        string `json:"type,omitempty"`
	Enum        string `json:"enum,omitempty"`
	FieldType   string `json:"fieldType,omitempty"`
	Description string `json:"description,omitempty"`
}

// SchemaSearchOutput is the output for the schema_search tool.
type SchemaSearchOutput struct {
	TotalMatches int                  `json:"totalMatches"`
	Results      []SchemaSearchResult `json:"results"`
}

// SchemaGetImplementorsInput is the input for the schema_get_implementors tool.
type SchemaGetImplementorsInput struct {
	Interface string `json:"interface"`
}

// SchemaImplementorInfo describes a type that implements an interface.
type SchemaImplementorInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SchemaGetImplementorsOutput is the output for the schema_get_implementors tool.
type SchemaGetImplementorsOutput struct {
	Interface    string                  `json:"interface"`
	Description  string                  `json:"description,omitempty"`
	Implementors []SchemaImplementorInfo `json:"implementors"`
	Count        int                     `json:"count"`
}

// SchemaGetUnionTypesInput is the input for the schema_get_union_types tool.
type SchemaGetUnionTypesInput struct {
	Union string `json:"union"`
}

// SchemaUnionMember describes a member type of a GraphQL union.
type SchemaUnionMember struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// SchemaGetUnionTypesOutput is the output for the schema_get_union_types tool.
type SchemaGetUnionTypesOutput struct {
	Union       string              `json:"union"`
	Description string              `json:"description,omitempty"`
	Types       []SchemaUnionMember `json:"types"`
	Count       int                 `json:"count"`
}

// =============================================================================
// GraphQL Tool Types
// =============================================================================

// NaisTeamInfo contains information about a team.
type NaisTeamInfo struct {
	Slug    string `json:"slug"`
	Purpose string `json:"purpose,omitempty"`
	Role    string `json:"role"`
}

// NaisUserInfo contains information about a user.
type NaisUserInfo struct {
	Name string `json:"name"`
}

// GetNaisContextOutput is the output for the get_nais_context tool.
type GetNaisContextOutput struct {
	User               NaisUserInfo      `json:"user"`
	Teams              []NaisTeamInfo    `json:"teams"`
	ConsoleBaseURL     string            `json:"console_base_url"`
	ConsoleURLPatterns map[string]string `json:"console_url_patterns"`
}

// ExecuteGraphQLInput is the input for the execute_graphql tool.
type ExecuteGraphQLInput struct {
	Query     string `json:"query"`
	Variables string `json:"variables,omitempty"`
}

// ValidateGraphQLInput is the input for the validate_graphql tool.
type ValidateGraphQLInput struct {
	Query string `json:"query"`
}

// ValidateGraphQLOutput is the output for the validate_graphql tool.
type ValidateGraphQLOutput struct {
	Valid         bool   `json:"valid"`
	Error         string `json:"error,omitempty"`
	OperationType string `json:"operationType,omitempty"`
	OperationName string `json:"operationName,omitempty"`
	Depth         int    `json:"depth,omitempty"`
}

// QueryValidationResult is the internal result of query validation.
type QueryValidationResult struct {
	Valid         bool
	Error         string
	OperationType string
	OperationName string
	Depth         int
}
