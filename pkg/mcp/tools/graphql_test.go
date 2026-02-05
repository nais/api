package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// mockGraphQLClient is a mock implementation of GraphQLClient for testing.
type mockGraphQLClient struct {
	executeResult map[string]any
	executeError  error
	user          *UserInfo
	userError     error
	teams         []TeamInfo
	teamsError    error
}

func (m *mockGraphQLClient) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	if m.executeError != nil {
		return nil, m.executeError
	}
	return m.executeResult, nil
}

func (m *mockGraphQLClient) GetCurrentUser(ctx context.Context) (*UserInfo, error) {
	if m.userError != nil {
		return nil, m.userError
	}
	return m.user, nil
}

func (m *mockGraphQLClient) GetUserTeams(ctx context.Context) ([]TeamInfo, error) {
	if m.teamsError != nil {
		return nil, m.teamsError
	}
	return m.teams, nil
}

func createTestSchemaForGraphQL() *ast.Schema {
	schemaSDL := `
type Query {
	team(slug: String!): Team
	me: AuthenticatedUser
}

type Mutation {
	createTeam(input: CreateTeamInput!): Team
}

union AuthenticatedUser = User | ServiceAccount

type User {
	email: String!
	name: String
	teams(first: Int): TeamMemberConnection
}

type ServiceAccount {
	name: String!
}

type Team {
	slug: String!
	purpose: String
	applications(first: Int): ApplicationConnection
	secrets(first: Int): SecretConnection
}

type Application {
	name: String!
	state: ApplicationState!
}

type Secret {
	name: String!
	data: SecretValue!
}

type SecretValue {
	value: String!
}

type SecretConnection {
	nodes: [Secret!]!
}

type ApplicationConnection {
	nodes: [Application!]!
}

type TeamMemberConnection {
	nodes: [TeamMember!]!
}

type TeamMember {
	team: Team!
	role: String!
}

enum ApplicationState {
	RUNNING
	NOT_RUNNING
	UNKNOWN
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

func TestGraphQLTools_GetNaisContext(t *testing.T) {
	schema := createTestSchemaForGraphQL()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		client := &mockGraphQLClient{
			user: &UserInfo{
				Name:    "Test User",
				IsAdmin: false,
			},
			teams: []TeamInfo{
				{Slug: "team-a", Purpose: "Team A purpose", Role: "owner"},
				{Slug: "team-b", Purpose: "Team B purpose", Role: "member"},
			},
		}

		tools := NewGraphQLTools(client, schema, "https://console.example.cloud.nais.io", map[string]string{
			"team": "/team/{team}",
		})

		result, err := tools.GetNaisContext(ctx, GetNaisContextInput{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.User.Name != "Test User" {
			t.Errorf("expected user name 'Test User', got '%s'", result.User.Name)
		}
		if len(result.Teams) != 2 {
			t.Errorf("expected 2 teams, got %d", len(result.Teams))
		}
		if result.ConsoleBaseURL != "https://console.example.cloud.nais.io" {
			t.Errorf("expected console URL, got '%s'", result.ConsoleBaseURL)
		}
		if result.ConsoleURLPatterns["team"] != "/team/{team}" {
			t.Errorf("expected URL pattern, got '%s'", result.ConsoleURLPatterns["team"])
		}
	})

	t.Run("user error", func(t *testing.T) {
		client := &mockGraphQLClient{
			userError: errors.New("auth failed"),
		}

		tools := NewGraphQLTools(client, schema, "https://console.example.cloud.nais.io", nil)

		_, err := tools.GetNaisContext(ctx, GetNaisContextInput{})
		if err == nil {
			t.Error("expected error, got none")
		}
	})

	t.Run("teams error", func(t *testing.T) {
		client := &mockGraphQLClient{
			user: &UserInfo{
				Name: "Test User",
			},
			teamsError: errors.New("teams fetch failed"),
		}

		tools := NewGraphQLTools(client, schema, "https://console.example.cloud.nais.io", nil)

		_, err := tools.GetNaisContext(ctx, GetNaisContextInput{})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}

func TestGraphQLTools_ExecuteGraphQL(t *testing.T) {
	schema := createTestSchemaForGraphQL()
	ctx := context.Background()

	t.Run("valid query", func(t *testing.T) {
		client := &mockGraphQLClient{
			executeResult: map[string]any{
				"team": map[string]any{
					"slug":    "my-team",
					"purpose": "Test purpose",
				},
			},
		}

		tools := NewGraphQLTools(client, schema, "", nil)

		result, err := tools.ExecuteGraphQL(ctx, ExecuteGraphQLInput{
			Query: `query { team(slug: "my-team") { slug purpose } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		teamData, ok := result["team"].(map[string]any)
		if !ok {
			t.Fatal("expected team in result")
		}
		if teamData["slug"] != "my-team" {
			t.Errorf("expected slug 'my-team', got '%v'", teamData["slug"])
		}
	})

	t.Run("query with variables", func(t *testing.T) {
		client := &mockGraphQLClient{
			executeResult: map[string]any{
				"team": map[string]any{
					"slug": "my-team",
				},
			},
		}

		tools := NewGraphQLTools(client, schema, "", nil)

		result, err := tools.ExecuteGraphQL(ctx, ExecuteGraphQLInput{
			Query:     `query($slug: String!) { team(slug: $slug) { slug } }`,
			Variables: `{"slug": "my-team"}`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result["team"] == nil {
			t.Error("expected team in result")
		}
	})

	t.Run("invalid variables JSON", func(t *testing.T) {
		client := &mockGraphQLClient{}
		tools := NewGraphQLTools(client, schema, "", nil)

		_, err := tools.ExecuteGraphQL(ctx, ExecuteGraphQLInput{
			Query:     `query { team(slug: "test") { slug } }`,
			Variables: `{invalid json}`,
		})
		if err == nil {
			t.Error("expected error for invalid JSON, got none")
		}
	})

	t.Run("mutation not allowed", func(t *testing.T) {
		client := &mockGraphQLClient{}
		tools := NewGraphQLTools(client, schema, "", nil)

		_, err := tools.ExecuteGraphQL(ctx, ExecuteGraphQLInput{
			Query: `mutation { createTeam(input: {slug: "new-team"}) { slug } }`,
		})
		if err == nil {
			t.Error("expected error for mutation, got none")
		}
	})

	t.Run("forbidden type - secrets", func(t *testing.T) {
		client := &mockGraphQLClient{}
		tools := NewGraphQLTools(client, schema, "", nil)

		_, err := tools.ExecuteGraphQL(ctx, ExecuteGraphQLInput{
			Query: `query { team(slug: "test") { secrets { nodes { name } } } }`,
		})
		if err == nil {
			t.Error("expected error for secret access, got none")
		}
	})

	t.Run("client error", func(t *testing.T) {
		client := &mockGraphQLClient{
			executeError: errors.New("graphql error"),
		}
		tools := NewGraphQLTools(client, schema, "", nil)

		_, err := tools.ExecuteGraphQL(ctx, ExecuteGraphQLInput{
			Query: `query { team(slug: "test") { slug } }`,
		})
		if err == nil {
			t.Error("expected error, got none")
		}
	})
}

func TestGraphQLTools_ValidateGraphQL(t *testing.T) {
	schema := createTestSchemaForGraphQL()
	ctx := context.Background()
	client := &mockGraphQLClient{}
	tools := NewGraphQLTools(client, schema, "", nil)

	t.Run("valid query", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test") { slug purpose } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Valid {
			t.Errorf("expected valid, got invalid: %s", result.Error)
		}
		if result.OperationType != "query" {
			t.Errorf("expected operation type 'query', got '%s'", result.OperationType)
		}
	})

	t.Run("named query", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query GetTeam { team(slug: "test") { slug } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !result.Valid {
			t.Errorf("expected valid, got invalid: %s", result.Error)
		}
		if result.OperationName != "GetTeam" {
			t.Errorf("expected operation name 'GetTeam', got '%s'", result.OperationName)
		}
	})

	t.Run("invalid syntax", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test" { slug } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid, got valid")
		}
		if result.Error == "" {
			t.Error("expected error message")
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test") { unknownField } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid, got valid")
		}
	})

	t.Run("mutation not allowed", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `mutation { createTeam(input: {slug: "test"}) { slug } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid for mutation, got valid")
		}
		if result.Error == "" {
			t.Error("expected error message about mutations not allowed")
		}
	})

	t.Run("forbidden type access", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test") { secrets { nodes { name data { value } } } } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Valid {
			t.Error("expected invalid for secret access, got valid")
		}
	})

	t.Run("query depth check", func(t *testing.T) {
		result, err := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test") { slug } }`,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Depth == 0 {
			t.Error("expected depth > 0")
		}
	})
}

func TestQueryValidation_Depth(t *testing.T) {
	schema := createTestSchemaForGraphQL()
	client := &mockGraphQLClient{}
	tools := NewGraphQLTools(client, schema, "", nil)
	ctx := context.Background()

	t.Run("simple query depth", func(t *testing.T) {
		result, _ := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test") { slug } }`,
		})

		if result.Depth != 2 {
			t.Errorf("expected depth 2, got %d", result.Depth)
		}
	})

	t.Run("nested query depth", func(t *testing.T) {
		result, _ := tools.ValidateGraphQL(ctx, ValidateGraphQLInput{
			Query: `query { team(slug: "test") { applications { nodes { name state } } } }`,
		})

		if result.Depth < 3 {
			t.Errorf("expected depth >= 3, got %d", result.Depth)
		}
	})
}

func TestNaisAPIGuidance(t *testing.T) {
	// Test that the guidance constant is not empty and contains key information
	if NaisAPIGuidance == "" {
		t.Error("NaisAPIGuidance should not be empty")
	}

	// Check for key sections
	sections := []string{
		"Nais API Guidance",
		"Key Concepts",
		"Common Query Patterns",
		"Pagination",
		"Tips",
	}

	for _, section := range sections {
		if !containsString(NaisAPIGuidance, section) {
			t.Errorf("NaisAPIGuidance should contain section: %s", section)
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
