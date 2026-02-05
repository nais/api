package tools

// =============================================================================
// Schema Tool Types
// =============================================================================

// --- schema_list_types ---

// SchemaListTypesInput is the input for the schema_list_types tool.
type SchemaListTypesInput struct {
	Kind   string `json:"kind,omitempty" jsonschema:"description=Filter by kind: 'OBJECT'\\, 'INTERFACE'\\, 'ENUM'\\, 'UNION'\\, 'INPUT_OBJECT'\\, 'SCALAR'\\, or 'all' (default: 'all')"`
	Search string `json:"search,omitempty" jsonschema:"description=Filter type names containing this string (case-insensitive)"`
}

// SchemaListTypesOutput is the output for the schema_list_types tool.
type SchemaListTypesOutput struct {
	Objects      []string `json:"objects,omitempty" jsonschema:"description=List of object type names"`
	Interfaces   []string `json:"interfaces,omitempty" jsonschema:"description=List of interface type names"`
	Enums        []string `json:"enums,omitempty" jsonschema:"description=List of enum type names"`
	Unions       []string `json:"unions,omitempty" jsonschema:"description=List of union type names"`
	InputObjects []string `json:"input_objects,omitempty" jsonschema:"description=List of input object type names"`
	Scalars      []string `json:"scalars,omitempty" jsonschema:"description=List of scalar type names"`
}

// --- schema_get_type ---

// SchemaGetTypeInput is the input for the schema_get_type tool.
type SchemaGetTypeInput struct {
	Name string `json:"name" jsonschema:"required,description=The exact type name (e.g.\\, 'Application'\\, 'Team'\\, 'DeploymentState')"`
}

// SchemaFieldInfo describes a field on a GraphQL type.
type SchemaFieldInfo struct {
	Name        string `json:"name" jsonschema:"description=Field name"`
	Type        string `json:"type" jsonschema:"description=Field type"`
	Description string `json:"description,omitempty" jsonschema:"description=Field description"`
	Deprecated  any    `json:"deprecated,omitempty" jsonschema:"description=Deprecation info if deprecated"`
}

// SchemaEnumValue describes a value in a GraphQL enum.
type SchemaEnumValue struct {
	Name        string `json:"name" jsonschema:"description=Enum value name"`
	Description string `json:"description,omitempty" jsonschema:"description=Enum value description"`
	Deprecated  any    `json:"deprecated,omitempty" jsonschema:"description=Deprecation info if deprecated"`
}

// SchemaGetTypeOutput is the output for the schema_get_type tool.
type SchemaGetTypeOutput struct {
	Name          string            `json:"name" jsonschema:"description=Type name"`
	Kind          string            `json:"kind" jsonschema:"description=Type kind (OBJECT\\, INTERFACE\\, ENUM\\, etc.)"`
	Description   string            `json:"description,omitempty" jsonschema:"description=Type description"`
	Implements    []string          `json:"implements,omitempty" jsonschema:"description=Interfaces this type implements"`
	Fields        []SchemaFieldInfo `json:"fields,omitempty" jsonschema:"description=Fields on this type"`
	Values        []SchemaEnumValue `json:"values,omitempty" jsonschema:"description=Enum values (for ENUM types)"`
	Types         []string          `json:"types,omitempty" jsonschema:"description=Member types (for UNION types)"`
	ImplementedBy []string          `json:"implementedBy,omitempty" jsonschema:"description=Types implementing this interface"`
}

// --- schema_list_queries ---

// SchemaListQueriesInput is the input for the schema_list_queries tool.
type SchemaListQueriesInput struct {
	Search string `json:"search,omitempty" jsonschema:"description=Filter query names or descriptions containing this string (case-insensitive)"`
}

// SchemaOperationInfo describes a GraphQL operation (query or mutation).
type SchemaOperationInfo struct {
	Name        string `json:"name" jsonschema:"description=Operation name"`
	ReturnType  string `json:"returnType" jsonschema:"description=Return type"`
	Description string `json:"description,omitempty" jsonschema:"description=Operation description"`
	ArgCount    int    `json:"argCount" jsonschema:"description=Number of arguments"`
}

// --- schema_list_mutations ---

// SchemaListMutationsInput is the input for the schema_list_mutations tool.
type SchemaListMutationsInput struct {
	Search string `json:"search,omitempty" jsonschema:"description=Filter mutation names or descriptions containing this string (case-insensitive)"`
}

// --- schema_get_field ---

// SchemaGetFieldInput is the input for the schema_get_field tool.
type SchemaGetFieldInput struct {
	Type  string `json:"type" jsonschema:"required,description=The type name containing the field (use 'Query' for root queries\\, 'Mutation' for root mutations\\, or any object type name)"`
	Field string `json:"field" jsonschema:"required,description=The field name to inspect"`
}

// SchemaArgumentInfo describes an argument on a GraphQL field.
type SchemaArgumentInfo struct {
	Name        string `json:"name" jsonschema:"description=Argument name"`
	Type        string `json:"type" jsonschema:"description=Argument type"`
	Description string `json:"description,omitempty" jsonschema:"description=Argument description"`
	Default     string `json:"default,omitempty" jsonschema:"description=Default value if any"`
}

// SchemaGetFieldOutput is the output for the schema_get_field tool.
type SchemaGetFieldOutput struct {
	Name        string               `json:"name" jsonschema:"description=Field name"`
	Type        string               `json:"type" jsonschema:"description=Field return type"`
	Description string               `json:"description,omitempty" jsonschema:"description=Field description"`
	Deprecated  any                  `json:"deprecated,omitempty" jsonschema:"description=Deprecation info if deprecated"`
	Args        []SchemaArgumentInfo `json:"args,omitempty" jsonschema:"description=Field arguments"`
}

// --- schema_get_enum ---

// SchemaGetEnumInput is the input for the schema_get_enum tool.
type SchemaGetEnumInput struct {
	Name string `json:"name" jsonschema:"required,description=The enum type name (e.g.\\, 'ApplicationState'\\, 'TeamRole')"`
}

// SchemaGetEnumOutput is the output for the schema_get_enum tool.
type SchemaGetEnumOutput struct {
	Name        string            `json:"name" jsonschema:"description=Enum name"`
	Description string            `json:"description,omitempty" jsonschema:"description=Enum description"`
	Values      []SchemaEnumValue `json:"values" jsonschema:"description=Enum values"`
}

// --- schema_search ---

// SchemaSearchInput is the input for the schema_search tool.
type SchemaSearchInput struct {
	Query string `json:"query" jsonschema:"required,description=Search term to match against names and descriptions (case-insensitive)"`
}

// SchemaSearchResult is a single result from a schema search.
type SchemaSearchResult struct {
	Kind        string `json:"kind" jsonschema:"description=Result kind (object\\, interface\\, field\\, enum_value\\, etc.)"`
	Name        string `json:"name" jsonschema:"description=Name of the matched item"`
	Type        string `json:"type,omitempty" jsonschema:"description=Parent type (for fields)"`
	Enum        string `json:"enum,omitempty" jsonschema:"description=Parent enum (for enum values)"`
	FieldType   string `json:"fieldType,omitempty" jsonschema:"description=Field type (for fields)"`
	Description string `json:"description,omitempty" jsonschema:"description=Description"`
}

// SchemaSearchOutput is the output for the schema_search tool.
type SchemaSearchOutput struct {
	TotalMatches int                  `json:"totalMatches" jsonschema:"description=Total number of matches found"`
	Results      []SchemaSearchResult `json:"results" jsonschema:"description=Search results (max 50)"`
}

// --- schema_get_implementors ---

// SchemaGetImplementorsInput is the input for the schema_get_implementors tool.
type SchemaGetImplementorsInput struct {
	Interface string `json:"interface" jsonschema:"required,description=The interface name (e.g.\\, 'Workload'\\, 'Issue')"`
}

// SchemaImplementorInfo describes a type that implements an interface.
type SchemaImplementorInfo struct {
	Name        string `json:"name" jsonschema:"description=Type name"`
	Description string `json:"description,omitempty" jsonschema:"description=Type description"`
}

// SchemaGetImplementorsOutput is the output for the schema_get_implementors tool.
type SchemaGetImplementorsOutput struct {
	Interface    string                  `json:"interface" jsonschema:"description=Interface name"`
	Description  string                  `json:"description,omitempty" jsonschema:"description=Interface description"`
	Implementors []SchemaImplementorInfo `json:"implementors" jsonschema:"description=Types implementing this interface"`
	Count        int                     `json:"count" jsonschema:"description=Number of implementors"`
}

// --- schema_get_union_types ---

// SchemaGetUnionTypesInput is the input for the schema_get_union_types tool.
type SchemaGetUnionTypesInput struct {
	Union string `json:"union" jsonschema:"required,description=The union type name"`
}

// SchemaUnionMember describes a member type of a GraphQL union.
type SchemaUnionMember struct {
	Name        string `json:"name" jsonschema:"description=Member type name"`
	Description string `json:"description,omitempty" jsonschema:"description=Member type description"`
}

// SchemaGetUnionTypesOutput is the output for the schema_get_union_types tool.
type SchemaGetUnionTypesOutput struct {
	Union       string              `json:"union" jsonschema:"description=Union name"`
	Description string              `json:"description,omitempty" jsonschema:"description=Union description"`
	Types       []SchemaUnionMember `json:"types" jsonschema:"description=Member types"`
	Count       int                 `json:"count" jsonschema:"description=Number of member types"`
}

// =============================================================================
// GraphQL Tool Types
// =============================================================================

// --- get_nais_context ---

// GetNaisContextInput is the input for the get_nais_context tool.
// This tool requires no input parameters.
type GetNaisContextInput struct{}

// NaisTeamInfo contains information about a team.
type NaisTeamInfo struct {
	Slug    string `json:"slug" jsonschema:"description=Team slug identifier"`
	Purpose string `json:"purpose,omitempty" jsonschema:"description=Team purpose"`
	Role    string `json:"role" jsonschema:"description=User's role in the team"`
}

// NaisUserInfo contains information about a user.
type NaisUserInfo struct {
	Name string `json:"name" jsonschema:"description=User's name"`
}

// GetNaisContextOutput is the output for the get_nais_context tool.
type GetNaisContextOutput struct {
	User               NaisUserInfo      `json:"user" jsonschema:"description=Current user info"`
	Teams              []NaisTeamInfo    `json:"teams" jsonschema:"description=User's teams"`
	ConsoleBaseURL     string            `json:"console_base_url" jsonschema:"description=Base URL for Nais console"`
	ConsoleURLPatterns map[string]string `json:"console_url_patterns" jsonschema:"description=URL patterns for console pages"`
}

// --- execute_graphql ---

// ExecuteGraphQLInput is the input for the execute_graphql tool.
type ExecuteGraphQLInput struct {
	Query     string `json:"query" jsonschema:"required,description=The GraphQL query to execute. Must be a query operation (not mutation or subscription)."`
	Variables string `json:"variables,omitempty" jsonschema:"description=JSON object containing variables for the query. Example: {\"slug\": \"my-team\"\\, \"first\": 10}"`
}

// --- validate_graphql ---

// ValidateGraphQLInput is the input for the validate_graphql tool.
type ValidateGraphQLInput struct {
	Query string `json:"query" jsonschema:"required,description=The GraphQL query to validate."`
}

// ValidateGraphQLOutput is the output for the validate_graphql tool.
type ValidateGraphQLOutput struct {
	Valid         bool   `json:"valid" jsonschema:"description=Whether the query is valid"`
	Error         string `json:"error,omitempty" jsonschema:"description=Validation error message if invalid"`
	OperationType string `json:"operationType,omitempty" jsonschema:"description=Type of operation (query\\, mutation\\, subscription)"`
	OperationName string `json:"operationName,omitempty" jsonschema:"description=Name of the operation if provided"`
	Depth         int    `json:"depth,omitempty" jsonschema:"description=Query depth"`
}
