package agent

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"github.com/sirupsen/logrus"
)

// mockAuthenticatedUser implements authz.AuthenticatedUser for testing
type mockAuthenticatedUser struct {
	id               uuid.UUID
	identity         string
	isServiceAccount bool
	isAdmin          bool
}

func (m *mockAuthenticatedUser) GetID() uuid.UUID                                  { return m.id }
func (m *mockAuthenticatedUser) Identity() string                                  { return m.identity }
func (m *mockAuthenticatedUser) IsServiceAccount() bool                            { return m.isServiceAccount }
func (m *mockAuthenticatedUser) IsAdmin() bool                                     { return m.isAdmin }
func (m *mockAuthenticatedUser) GCPTeamGroups(_ context.Context) ([]string, error) { return nil, nil }

func TestInternalClient_GetCurrentUser(t *testing.T) {
	// Create a mock user
	mockUser := &mockAuthenticatedUser{
		id:       uuid.New(),
		identity: "test@example.com",
		isAdmin:  false,
	}

	// Create context with user
	ctx := authz.ContextWithActor(context.Background(), mockUser, nil)

	// Create client (handler is nil for this test since we don't call ExecuteGraphQL)
	client := &InternalClient{
		log: logrus.NewEntry(logrus.StandardLogger()),
	}

	// Test GetCurrentUser
	user, err := client.GetCurrentUser(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if user.Name != "test@example.com" {
		t.Errorf("expected name %q, got %q", "test@example.com", user.Name)
	}

	if user.IsAdmin {
		t.Error("expected user to not be admin")
	}
}

func TestInternalClient_GetCurrentUser_NoUser(t *testing.T) {
	// Create context without user
	ctx := context.Background()

	// Create client
	client := &InternalClient{
		log: logrus.NewEntry(logrus.StandardLogger()),
	}

	// Test GetCurrentUser
	_, err := client.GetCurrentUser(ctx)
	if err == nil {
		t.Error("expected error when no user in context")
	}
}

func TestInternalClient_GetCurrentUser_Admin(t *testing.T) {
	// Create a mock admin user
	mockUser := &mockAuthenticatedUser{
		id:       uuid.New(),
		identity: "admin@example.com",
		isAdmin:  true,
	}

	// Create context with user
	ctx := authz.ContextWithActor(context.Background(), mockUser, nil)

	// Create client
	client := &InternalClient{
		log: logrus.NewEntry(logrus.StandardLogger()),
	}

	// Test GetCurrentUser
	user, err := client.GetCurrentUser(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !user.IsAdmin {
		t.Error("expected user to be admin")
	}
}

func TestMCPClientAdapter(t *testing.T) {
	// Create a mock client
	mockClient := &mockMCPClient{
		user: &MCPUserInfo{
			Name:    "Test User",
			IsAdmin: false,
		},
		teams: []MCPTeamInfo{
			{Slug: "team1", Purpose: "Test team 1", Role: "owner"},
			{Slug: "team2", Purpose: "Test team 2", Role: "member"},
		},
	}

	// Create adapter
	adapter := NewMCPClientAdapter(mockClient)

	ctx := context.Background()

	// Test ExecuteGraphQL
	result, err := adapter.ExecuteGraphQL(ctx, "query { test }", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["test"] != "value" {
		t.Errorf("expected test=value, got %v", result)
	}

	// Test GetCurrentUser
	userResult, err := adapter.GetCurrentUser(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userResult == nil {
		t.Error("expected user result")
	}

	// Test GetUserTeams
	teamsResult, err := adapter.GetUserTeams(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if teamsResult == nil {
		t.Error("expected teams result")
	}
}

// mockMCPClient implements MCPClient for testing
type mockMCPClient struct {
	user  *MCPUserInfo
	teams []MCPTeamInfo
}

func (m *mockMCPClient) ExecuteGraphQL(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	return map[string]any{"test": "value"}, nil
}

func (m *mockMCPClient) GetCurrentUser(_ context.Context) (*MCPUserInfo, error) {
	return m.user, nil
}

func (m *mockMCPClient) GetUserTeams(_ context.Context) ([]MCPTeamInfo, error) {
	return m.teams, nil
}

// Ensure mockMCPClient implements MCPClient
var _ MCPClient = (*mockMCPClient)(nil)

// Ensure mockAuthenticatedUser implements authz.AuthenticatedUser
var _ authz.AuthenticatedUser = (*mockAuthenticatedUser)(nil)

// Dummy reference to slug package to avoid unused import error in tests
var _ = slug.Slug("")
