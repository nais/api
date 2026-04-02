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
	"github.com/nais/api/internal/agent/tools"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/sirupsen/logrus"
)

// InternalClient executes GraphQL queries directly against the internal handler
// without going through a real HTTP connection. This is used by the tool integration
// to query the Nais API on behalf of users.
//
// InternalClient implements tools.GraphQLClient directly.
type InternalClient struct {
	handler *handler.Server
	log     logrus.FieldLogger
}

// Ensure InternalClient implements tools.GraphQLClient.
var _ tools.GraphQLClient = (*InternalClient)(nil)

// NewInternalClient creates a new InternalClient for executing GraphQL queries
// directly against the internal handler.
func NewInternalClient(h *handler.Server, log logrus.FieldLogger) *InternalClient {
	if log == nil {
		log = logrus.StandardLogger()
	}
	return &InternalClient{
		handler: h,
		log:     log,
	}
}

// ExecuteGraphQL runs a GraphQL query with the given variables.
// Returns the data portion of the response as a map.
func (c *InternalClient) ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error) {
	c.log.WithFields(logrus.Fields{
		"query_length":  len(query),
		"has_variables": variables != nil,
	}).Debug("executing internal GraphQL query")

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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)

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

	if len(result.Errors) > 0 {
		errMsgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			errMsgs[i] = e.Message
		}
		c.log.WithField("errors", errMsgs).Warn("GraphQL query returned errors")
		if result.Data == nil {
			return nil, fmt.Errorf("GraphQL errors: %v", errMsgs)
		}
	}

	return result.Data, nil
}

// GetCurrentUser returns information about the current authenticated user.
func (c *InternalClient) GetCurrentUser(ctx context.Context) (*tools.UserInfo, error) {
	c.log.Debug("getting current user from context")

	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return nil, fmt.Errorf("no authenticated user in context")
	}

	return &tools.UserInfo{
		Name:    actor.User.Identity(),
		IsAdmin: actor.User.IsAdmin(),
	}, nil
}

// GetUserTeams returns the teams the current user belongs to.
func (c *InternalClient) GetUserTeams(ctx context.Context) ([]tools.TeamInfo, error) {
	c.log.Debug("getting user teams")

	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return nil, fmt.Errorf("no authenticated user in context")
	}

	userID := actor.User.GetID()

	first := 100
	page, err := pagination.ParsePage(&first, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pagination: %w", err)
	}

	teamsResult, err := team.ListForUser(ctx, userID, page, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams for user: %w", err)
	}

	nodes := teamsResult.Nodes()
	result := make([]tools.TeamInfo, 0, len(nodes))
	for _, member := range nodes {
		t, err := team.Get(ctx, member.TeamSlug)
		if err != nil {
			c.log.WithError(err).WithField("team_slug", member.TeamSlug).Warn("failed to get team details")
			continue
		}

		result = append(result, tools.TeamInfo{
			Slug:    string(member.TeamSlug),
			Purpose: t.Purpose,
			Role:    string(member.Role),
		})
	}

	c.log.WithField("team_count", len(result)).Debug("retrieved user teams")
	return result, nil
}
