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

// InternalUserInfo contains information about an authenticated user.
type InternalUserInfo struct {
	// Name is the user's display name or email.
	Name string

	// IsAdmin indicates whether the user has admin privileges.
	IsAdmin bool
}

// InternalTeamInfo contains information about a team the user belongs to.
type InternalTeamInfo struct {
	// Slug is the team's unique identifier.
	Slug string

	// Purpose describes what the team does.
	Purpose string

	// Role is the user's role in the team (e.g., "owner", "member").
	Role string
}

// InternalClient executes GraphQL queries directly against the internal handler
// without going through a real HTTP connection. This is used by the tool integration
// to query the Nais API on behalf of users.
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
func (c *InternalClient) GetCurrentUser(ctx context.Context) (*InternalUserInfo, error) {
	c.log.Debug("getting current user from context")

	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return nil, fmt.Errorf("no authenticated user in context")
	}

	return &InternalUserInfo{
		Name:    actor.User.Identity(),
		IsAdmin: actor.User.IsAdmin(),
	}, nil
}

// GetUserTeams returns the teams the current user belongs to.
func (c *InternalClient) GetUserTeams(ctx context.Context) ([]InternalTeamInfo, error) {
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

	// Convert to InternalTeamInfo
	nodes := teamsResult.Nodes()
	result := make([]InternalTeamInfo, 0, len(nodes))
	for _, member := range nodes {
		// Get team details
		t, err := team.Get(ctx, member.TeamSlug)
		if err != nil {
			c.log.WithError(err).WithField("team_slug", member.TeamSlug).Warn("failed to get team details")
			continue
		}

		result = append(result, InternalTeamInfo{
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
