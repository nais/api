// Package tools provides tool definitions and execution for the agent.
package tools

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// NaisAPIGuidance provides context about the API structure and common patterns for LLMs.
//
//go:embed nais_api_guidance.md
var NaisAPIGuidance string

// GraphQLClient is the interface for executing GraphQL queries.
type GraphQLClient interface {
	// ExecuteGraphQL runs a GraphQL query with the given variables.
	ExecuteGraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error)

	// GetCurrentUser returns information about the current authenticated user.
	GetCurrentUser(ctx context.Context) (*UserInfo, error)

	// GetUserTeams returns the teams the current user belongs to.
	GetUserTeams(ctx context.Context) ([]TeamInfo, error)
}

// UserInfo contains information about an authenticated user.
type UserInfo struct {
	Name    string
	IsAdmin bool
}

// TeamInfo contains information about a team the user belongs to.
type TeamInfo struct {
	Slug    string
	Purpose string
	Role    string
}

// GraphQLTools provides GraphQL execution functionality.
type GraphQLTools struct {
	client         GraphQLClient
	schema         *ast.Schema
	consoleBaseURL string
	urlPatterns    map[string]string
}

// NewGraphQLTools creates a new GraphQLTools instance.
func NewGraphQLTools(client GraphQLClient, schema *ast.Schema, consoleBaseURL string, urlPatterns map[string]string) *GraphQLTools {
	return &GraphQLTools{
		client:         client,
		schema:         schema,
		consoleBaseURL: consoleBaseURL,
		urlPatterns:    urlPatterns,
	}
}

// GetNaisContext returns the current user, their teams, and console URL information.
func (g *GraphQLTools) GetNaisContext(ctx context.Context) (GetNaisContextOutput, error) {
	// Get current user
	user, err := g.client.GetCurrentUser(ctx)
	if err != nil {
		return GetNaisContextOutput{}, fmt.Errorf("failed to get current user: %w", err)
	}

	// Get user's teams
	teams, err := g.client.GetUserTeams(ctx)
	if err != nil {
		return GetNaisContextOutput{}, fmt.Errorf("failed to get user teams: %w", err)
	}

	// Build teams list
	teamsList := make([]NaisTeamInfo, 0, len(teams))
	for _, team := range teams {
		teamsList = append(teamsList, NaisTeamInfo{
			Slug:    team.Slug,
			Purpose: team.Purpose,
			Role:    team.Role,
		})
	}

	return GetNaisContextOutput{
		User: NaisUserInfo{
			Name: user.Name,
		},
		Teams:              teamsList,
		ConsoleBaseURL:     g.consoleBaseURL,
		ConsoleURLPatterns: g.urlPatterns,
	}, nil
}

// ExecuteGraphQL executes a GraphQL query after validation.
func (g *GraphQLTools) ExecuteGraphQL(ctx context.Context, input ExecuteGraphQLInput) (map[string]any, error) {
	variablesStr := input.Variables
	if variablesStr == "" {
		variablesStr = "{}"
	}

	// Parse variables
	var variables map[string]any
	if err := json.Unmarshal([]byte(variablesStr), &variables); err != nil {
		return nil, fmt.Errorf("invalid variables JSON: %w", err)
	}

	// Validate the query
	validationResult := g.validateQuery(input.Query)
	if !validationResult.Valid {
		return nil, fmt.Errorf("invalid query: %s", validationResult.Error)
	}

	// Execute the query
	result, err := g.client.ExecuteGraphQL(ctx, input.Query, variables)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	return result, nil
}

// ValidateGraphQL validates a GraphQL query without executing it.
func (g *GraphQLTools) ValidateGraphQL(ctx context.Context, input ValidateGraphQLInput) (ValidateGraphQLOutput, error) {
	result := g.validateQuery(input.Query)

	if result.Valid {
		return ValidateGraphQLOutput{
			Valid:         true,
			OperationType: result.OperationType,
			OperationName: result.OperationName,
			Depth:         result.Depth,
		}, nil
	}

	return ValidateGraphQLOutput{
		Valid: false,
		Error: result.Error,
	}, nil
}

// validateQuery validates a GraphQL query against the schema.
func (g *GraphQLTools) validateQuery(query string) *QueryValidationResult {
	// Parse the query against the schema
	doc, errList := gqlparser.LoadQuery(g.schema, query)
	if len(errList) > 0 {
		return &QueryValidationResult{
			Valid: false,
			Error: errList.Error(),
		}
	}

	// Check that we have at least one operation
	if len(doc.Operations) == 0 {
		return &QueryValidationResult{
			Valid: false,
			Error: "no operations found in query",
		}
	}

	// Check operation type - only allow queries
	op := doc.Operations[0]
	if op.Operation != ast.Query {
		return &QueryValidationResult{
			Valid: false,
			Error: fmt.Sprintf("only query operations are allowed, got: %s", op.Operation),
		}
	}

	// Check query depth
	depth := calculateQueryDepth(op.SelectionSet, 0)
	if depth > maxQueryDepth {
		return &QueryValidationResult{
			Valid: false,
			Error: fmt.Sprintf("query depth %d exceeds maximum allowed depth of %d", depth, maxQueryDepth),
		}
	}

	// Check for forbidden secret-related types and fields
	if found, reason := checkForSecrets(op.SelectionSet, g.schema); found {
		return &QueryValidationResult{
			Valid: false,
			Error: reason,
		}
	}

	return &QueryValidationResult{
		Valid:         true,
		OperationType: string(op.Operation),
		OperationName: op.Name,
		Depth:         depth,
	}
}
