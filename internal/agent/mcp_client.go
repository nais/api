// Package agent provides the AI chat service for the Nais platform.
package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/sirupsen/logrus"
)

// MCPClient defines the interface for GraphQL operations needed by MCP tools.
// This mirrors the mcp.Client interface from pkg/mcp but is defined here
// to avoid import cycle issues with the separate mcp module.
type MCPClient interface {
	// ExecuteGraphQL runs a GraphQL query with the given variables.
	// Returns the data portion of the response as a map.
	ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error)

	// GetCurrentUser returns information about the current authenticated user.
	GetCurrentUser(ctx context.Context) (*MCPUserInfo, error)

	// GetUserTeams returns the teams the current user belongs to.
	GetUserTeams(ctx context.Context) ([]MCPTeamInfo, error)
}

// MCPUserInfo contains information about an authenticated user.
type MCPUserInfo struct {
	// Name is the user's display name or email.
	Name string

	// IsAdmin indicates whether the user has admin privileges.
	IsAdmin bool
}

// MCPTeamInfo contains information about a team the user belongs to.
type MCPTeamInfo struct {
	// Slug is the team's unique identifier.
	Slug string

	// Purpose describes what the team does.
	Purpose string

	// Role is the user's role in the team (e.g., "owner", "member").
	Role string
}

// InternalClient implements the MCPClient interface for the hosted agent,
// executing GraphQL queries directly against the internal handler without
// going through a real HTTP connection.
type InternalClient struct {
	handler *handler.Server
	log     logrus.FieldLogger
}

// InternalClientConfig holds configuration for creating an InternalClient.
type InternalClientConfig struct {
	// Handler is the gqlgen handler server for executing GraphQL queries.
	Handler *handler.Server

	// Log is the logger for client operations.
	Log logrus.FieldLogger
}

// NewInternalClient creates a new InternalClient for executing GraphQL queries
// directly against the internal handler.
func NewInternalClient(cfg InternalClientConfig) *InternalClient {
	if cfg.Log == nil {
		cfg.Log = logrus.StandardLogger()
	}

	return &InternalClient{
		handler: cfg.Handler,
		log:     cfg.Log,
	}
}

// Ensure InternalClient implements MCPClient
var _ MCPClient = (*InternalClient)(nil)

// ExecuteGraphQL runs a GraphQL query with the given variables.
// Returns the data portion of the response as a map.
func (c *InternalClient) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	c.log.WithFields(logrus.Fields{
		"query_length":  len(query),
		"has_variables": variables != nil,
	}).Debug("executing internal GraphQL query")

	// Build the GraphQL request body
	requestBody := map[string]any{
		"query": query,
	}
	if variables != nil {
		requestBody["variables"] = variables
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Create a fake HTTP request to pass to the handler
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	rec := httptest.NewRecorder()

	// Execute the query through the handler
	c.handler.ServeHTTP(rec, req)

	// Read the response
	resp := rec.Result()
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	c.log.WithFields(logrus.Fields{
		"status_code":     resp.StatusCode,
		"response_length": len(respBody),
	}).Debug("GraphQL response received")

	// Parse the response
	var result struct {
		Data   map[string]any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
			Path    []any  `json:"path,omitempty"`
		} `json:"errors,omitempty"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to parse GraphQL response: %w", err)
	}

	// Check for GraphQL errors
	if len(result.Errors) > 0 {
		errMsgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			errMsgs[i] = e.Message
		}
		c.log.WithField("errors", errMsgs).Warn("GraphQL query returned errors")
		// Return the data along with errors - partial results are valid in GraphQL
		if result.Data == nil {
			return nil, fmt.Errorf("GraphQL errors: %v", errMsgs)
		}
	}

	return result.Data, nil
}

// GetCurrentUser returns information about the current authenticated user.
func (c *InternalClient) GetCurrentUser(ctx context.Context) (*MCPUserInfo, error) {
	c.log.Debug("getting current user from context")

	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return nil, fmt.Errorf("no authenticated user in context")
	}

	return &MCPUserInfo{
		Name:    actor.User.Identity(),
		IsAdmin: actor.User.IsAdmin(),
	}, nil
}

// GetUserTeams returns the teams the current user belongs to.
func (c *InternalClient) GetUserTeams(ctx context.Context) ([]MCPTeamInfo, error) {
	c.log.Debug("getting user teams")

	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return nil, fmt.Errorf("no authenticated user in context")
	}

	// Get the user's ID
	userID := actor.User.GetID()

	// Query teams for user - using a large page to get all teams
	first := 100
	page, err := pagination.ParsePage(&first, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pagination: %w", err)
	}

	teamsResult, err := team.ListForUser(ctx, userID, page, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams for user: %w", err)
	}

	// Convert to MCPTeamInfo
	nodes := teamsResult.Nodes()
	result := make([]MCPTeamInfo, 0, len(nodes))
	for _, member := range nodes {
		// Get team details
		t, err := team.Get(ctx, member.TeamSlug)
		if err != nil {
			c.log.WithError(err).WithField("team_slug", member.TeamSlug).Warn("failed to get team details")
			continue
		}

		result = append(result, MCPTeamInfo{
			Slug:    string(member.TeamSlug),
			Purpose: t.Purpose,
			Role:    string(member.Role),
		})
	}

	c.log.WithField("team_count", len(result)).Debug("retrieved user teams")
	return result, nil
}

// InternalClientFromHandler creates an InternalClient from a handler with minimal config.
// This is a convenience function for creating a client in tests or simple setups.
func InternalClientFromHandler(h *handler.Server) *InternalClient {
	return NewInternalClient(InternalClientConfig{
		Handler: h,
	})
}

// MCPClientAdapter adapts the internal MCPClient interface to the pkg/mcp.Client interface.
// This is useful when you need to pass an InternalClient to the mcp.Executor.
//
// Usage:
//
//	internalClient := agent.NewInternalClient(cfg)
//	adapter := agent.NewMCPClientAdapter(internalClient)
//	executor, _ := mcp.NewExecutor(
//	    mcp.WithClient(adapter),
//	    mcp.WithSchemaProvider(schemaProvider),
//	)
type MCPClientAdapter struct {
	client MCPClient
}

// NewMCPClientAdapter creates a new adapter wrapping an MCPClient.
func NewMCPClientAdapter(client MCPClient) *MCPClientAdapter {
	return &MCPClientAdapter{client: client}
}

// ExecuteGraphQL implements the mcp.Client interface.
func (a *MCPClientAdapter) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	return a.client.ExecuteGraphQL(ctx, query, variables)
}

// GetCurrentUser implements the mcp.Client interface.
// Note: The return type matches mcp.UserInfo structure.
func (a *MCPClientAdapter) GetCurrentUser(ctx context.Context) (any, error) {
	user, err := a.client.GetCurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	// Return a struct that matches mcp.UserInfo
	return &struct {
		Name    string
		IsAdmin bool
	}{
		Name:    user.Name,
		IsAdmin: user.IsAdmin,
	}, nil
}

// GetUserTeams implements the mcp.Client interface.
// Note: The return type matches []mcp.TeamInfo structure.
func (a *MCPClientAdapter) GetUserTeams(ctx context.Context) (any, error) {
	teams, err := a.client.GetUserTeams(ctx)
	if err != nil {
		return nil, err
	}
	// Return a slice that matches []mcp.TeamInfo
	result := make([]struct {
		Slug    string
		Purpose string
		Role    string
	}, len(teams))
	for i, t := range teams {
		result[i] = struct {
			Slug    string
			Purpose string
			Role    string
		}{
			Slug:    t.Slug,
			Purpose: t.Purpose,
			Role:    t.Role,
		}
	}
	return result, nil
}
