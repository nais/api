package tools

import (
	"context"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

func createTestSchema() *ast.Schema {
	schemaSDL := `
type Query {
	team(slug: String!): Team
	me: User
	search(query: String!, first: Int): SearchConnection
}

type Mutation {
	createTeam(input: CreateTeamInput!): Team
	deleteTeam(slug: String!): Boolean
}

type Team implements Node {
	id: ID!
	slug: String!
	purpose: String
	applications(first: Int, after: String): ApplicationConnection
	jobs(first: Int): JobConnection
}

type User implements Node {
	id: ID!
	email: String!
	name: String
	teams(first: Int): TeamMemberConnection
}

type Application implements Node & Workload {
	id: ID!
	name: String!
	state: ApplicationState!
	image: ContainerImage
}

type Job implements Node & Workload {
	id: ID!
	name: String!
	state: JobState!
	schedule: Schedule
}

type ContainerImage {
	name: String!
	tag: String!
}

type Schedule {
	expression: String!
}

type ApplicationConnection {
	nodes: [Application!]!
	pageInfo: PageInfo!
}

type JobConnection {
	nodes: [Job!]!
	pageInfo: PageInfo!
}

type TeamMemberConnection {
	nodes: [TeamMember!]!
	pageInfo: PageInfo!
}

type TeamMember {
	team: Team!
	role: TeamRole!
}

type SearchConnection {
	nodes: [SearchResult!]!
}

type PageInfo {
	hasNextPage: Boolean!
	endCursor: String
}

interface Node {
	id: ID!
}

interface Workload {
	name: String!
}

union SearchResult = Team | Application | Job | User

enum ApplicationState {
	RUNNING
	NOT_RUNNING
	UNKNOWN
}

enum JobState {
	RUNNING
	NOT_RUNNING
	UNKNOWN
}

enum TeamRole {
	OWNER
	MEMBER
}

input CreateTeamInput {
	slug: String!
	purpose: String
}
`
	schema, err := gqlparser.LoadSchema(&ast.Source{Name: "test.graphql", Input: schemaSDL})
	if err != nil {
		panic(err)
	}
	return schema
}

func TestSchemaTools_ListTypes(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("list all types", func(t *testing.T) {
		result, err := tools.ListTypes(ctx, SchemaListTypesInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Check that we have objects
		if len(result.Objects) == 0 {
			t.Error("expected objects, got none")
		}

		// Check that Team is in objects
		found := false
		for _, name := range result.Objects {
			if name == "Team" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected Team in objects")
		}

		// Check enums
		if len(result.Enums) == 0 {
			t.Error("expected enums, got none")
		}

		// Check interfaces
		if len(result.Interfaces) == 0 {
			t.Error("expected interfaces, got none")
		}

		// Check unions
		if len(result.Unions) == 0 {
			t.Error("expected unions, got none")
		}

		// Check input objects
		if len(result.InputObjects) == 0 {
			t.Error("expected input objects, got none")
		}
	})

	t.Run("filter by kind", func(t *testing.T) {
		result, err := tools.ListTypes(ctx, SchemaListTypesInput{Kind: "ENUM"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should only have enums
		if len(result.Objects) != 0 {
			t.Errorf("expected no objects, got %d", len(result.Objects))
		}
		if len(result.Enums) == 0 {
			t.Error("expected enums, got none")
		}
	})

	t.Run("filter by search", func(t *testing.T) {
		result, err := tools.ListTypes(ctx, SchemaListTypesInput{Search: "team"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should find Team-related types
		totalFound := len(result.Objects) + len(result.Enums) + len(result.Interfaces) + len(result.Unions) + len(result.InputObjects)
		if totalFound == 0 {
			t.Error("expected to find types matching 'team'")
		}
	})
}

func TestSchemaTools_GetType(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("get object type", func(t *testing.T) {
		result, err := tools.GetType(ctx, SchemaGetTypeInput{Name: "Team"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Name != "Team" {
			t.Errorf("expected name Team, got %s", result.Name)
		}
		if result.Kind != "OBJECT" {
			t.Errorf("expected kind OBJECT, got %s", result.Kind)
		}
		if len(result.Fields) == 0 {
			t.Error("expected fields, got none")
		}
		if len(result.Implements) == 0 {
			t.Error("expected implements, got none")
		}
	})

	t.Run("get enum type", func(t *testing.T) {
		result, err := tools.GetType(ctx, SchemaGetTypeInput{Name: "ApplicationState"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Kind != "ENUM" {
			t.Errorf("expected kind ENUM, got %s", result.Kind)
		}
		if len(result.Values) == 0 {
			t.Error("expected values, got none")
		}

		// Check for RUNNING value
		found := false
		for _, v := range result.Values {
			if v.Name == "RUNNING" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected RUNNING in enum values")
		}
	})

	t.Run("get interface type", func(t *testing.T) {
		result, err := tools.GetType(ctx, SchemaGetTypeInput{Name: "Node"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Kind != "INTERFACE" {
			t.Errorf("expected kind INTERFACE, got %s", result.Kind)
		}
		if len(result.ImplementedBy) == 0 {
			t.Error("expected implementedBy, got none")
		}
	})

	t.Run("get union type", func(t *testing.T) {
		result, err := tools.GetType(ctx, SchemaGetTypeInput{Name: "SearchResult"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Kind != "UNION" {
			t.Errorf("expected kind UNION, got %s", result.Kind)
		}
		if len(result.Types) == 0 {
			t.Error("expected types, got none")
		}
	})

	t.Run("type not found", func(t *testing.T) {
		_, err := tools.GetType(ctx, SchemaGetTypeInput{Name: "NonExistent"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}

func TestSchemaTools_ListQueries(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("list all queries", func(t *testing.T) {
		result, err := tools.ListQueries(ctx, SchemaListQueriesInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) == 0 {
			t.Error("expected queries, got none")
		}

		// Check for team query
		found := false
		for _, q := range result {
			if q.Name == "team" {
				found = true
				if q.ArgCount == 0 {
					t.Error("expected team query to have arguments")
				}
				break
			}
		}
		if !found {
			t.Error("expected team query")
		}
	})

	t.Run("filter queries by search", func(t *testing.T) {
		result, err := tools.ListQueries(ctx, SchemaListQueriesInput{Search: "team"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) == 0 {
			t.Error("expected queries matching 'team'")
		}
	})
}

func TestSchemaTools_ListMutations(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("list all mutations", func(t *testing.T) {
		result, err := tools.ListMutations(ctx, SchemaListMutationsInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result) == 0 {
			t.Error("expected mutations, got none")
		}

		// Check for createTeam mutation
		found := false
		for _, m := range result {
			if m.Name == "createTeam" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected createTeam mutation")
		}
	})
}

func TestSchemaTools_GetField(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("get query field", func(t *testing.T) {
		result, err := tools.GetField(ctx, SchemaGetFieldInput{Type: "Query", Field: "team"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Name != "team" {
			t.Errorf("expected name team, got %s", result.Name)
		}
		if len(result.Args) == 0 {
			t.Error("expected arguments, got none")
		}
	})

	t.Run("get type field", func(t *testing.T) {
		result, err := tools.GetField(ctx, SchemaGetFieldInput{Type: "Team", Field: "slug"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Name != "slug" {
			t.Errorf("expected name slug, got %s", result.Name)
		}
		if result.Type != "String!" {
			t.Errorf("expected type String!, got %s", result.Type)
		}
	})

	t.Run("field not found", func(t *testing.T) {
		_, err := tools.GetField(ctx, SchemaGetFieldInput{Type: "Team", Field: "nonexistent"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	t.Run("type not found", func(t *testing.T) {
		_, err := tools.GetField(ctx, SchemaGetFieldInput{Type: "NonExistent", Field: "slug"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}

func TestSchemaTools_GetEnum(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("get enum", func(t *testing.T) {
		result, err := tools.GetEnum(ctx, SchemaGetEnumInput{Name: "TeamRole"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Name != "TeamRole" {
			t.Errorf("expected name TeamRole, got %s", result.Name)
		}
		if len(result.Values) != 2 {
			t.Errorf("expected 2 values, got %d", len(result.Values))
		}
	})

	t.Run("enum not found", func(t *testing.T) {
		_, err := tools.GetEnum(ctx, SchemaGetEnumInput{Name: "NonExistent"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	t.Run("not an enum", func(t *testing.T) {
		_, err := tools.GetEnum(ctx, SchemaGetEnumInput{Name: "Team"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}

func TestSchemaTools_Search(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("search types", func(t *testing.T) {
		result, err := tools.Search(ctx, SchemaSearchInput{Query: "application"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.TotalMatches == 0 {
			t.Error("expected matches, got none")
		}
	})

	t.Run("search fields", func(t *testing.T) {
		result, err := tools.Search(ctx, SchemaSearchInput{Query: "slug"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should find slug field
		foundField := false
		for _, r := range result.Results {
			if r.Kind == "field" && r.Name == "slug" {
				foundField = true
				break
			}
		}
		if !foundField {
			t.Error("expected to find slug field")
		}
	})

	t.Run("search enum values", func(t *testing.T) {
		result, err := tools.Search(ctx, SchemaSearchInput{Query: "running"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should find RUNNING enum value
		foundValue := false
		for _, r := range result.Results {
			if r.Kind == "enum_value" && r.Name == "RUNNING" {
				foundValue = true
				break
			}
		}
		if !foundValue {
			t.Error("expected to find RUNNING enum value")
		}
	})
}

func TestSchemaTools_GetImplementors(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("get implementors", func(t *testing.T) {
		result, err := tools.GetImplementors(ctx, SchemaGetImplementorsInput{Interface: "Workload"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Interface != "Workload" {
			t.Errorf("expected interface Workload, got %s", result.Interface)
		}
		if result.Count == 0 {
			t.Error("expected implementors, got none")
		}

		// Check that Application implements Workload
		found := false
		for _, impl := range result.Implementors {
			if impl.Name == "Application" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected Application in implementors")
		}
	})

	t.Run("interface not found", func(t *testing.T) {
		_, err := tools.GetImplementors(ctx, SchemaGetImplementorsInput{Interface: "NonExistent"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	t.Run("not an interface", func(t *testing.T) {
		_, err := tools.GetImplementors(ctx, SchemaGetImplementorsInput{Interface: "Team"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}

func TestSchemaTools_GetUnionTypes(t *testing.T) {
	schema := createTestSchema()
	tools := NewSchemaTools(schema)
	ctx := context.Background()

	t.Run("get union types", func(t *testing.T) {
		result, err := tools.GetUnionTypes(ctx, SchemaGetUnionTypesInput{Union: "SearchResult"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Union != "SearchResult" {
			t.Errorf("expected union SearchResult, got %s", result.Union)
		}
		if result.Count == 0 {
			t.Error("expected types, got none")
		}

		// Check that Team is in the union
		found := false
		for _, typ := range result.Types {
			if typ.Name == "Team" {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected Team in union types")
		}
	})

	t.Run("union not found", func(t *testing.T) {
		_, err := tools.GetUnionTypes(ctx, SchemaGetUnionTypesInput{Union: "NonExistent"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	t.Run("not a union", func(t *testing.T) {
		_, err := tools.GetUnionTypes(ctx, SchemaGetUnionTypesInput{Union: "Team"})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}
