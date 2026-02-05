package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/vektah/gqlparser/v2"
	"github.com/vektah/gqlparser/v2/ast"
)

// Nais API guidance for LLMs - provides context about the API structure and common patterns
const NaisAPIGuidance = `
## Nais API Guidance

The Nais API is a GraphQL API for managing applications and jobs on the Nais platform.

### Key Concepts

- **Team**: The primary organizational unit. All resources belong to a team.
- **Application**: A long-running workload (deployment) managed by Nais (Often called only App/app).
- **Job**: A scheduled or one-off workload (CronJob/Job) managed by Nais.
- **Environment**: A Kubernetes cluster/namespace where workloads run (e.g., "dev", "prod").
- **Workload**: A union type representing either an Application or Job.

### Common Query Patterns

1. **Get current user and their teams**:
   ` + "```graphql" + `
   query { me { ... on User { teams(first: 50) { nodes { team { slug } } } } } }
   ` + "```" + `

2. **Get team details**:
   ` + "```graphql" + `
   query($slug: Slug!) { team(slug: $slug) { slug purpose slackChannel } }
   ` + "```" + `

3. **List applications for a team**:
   ` + "```graphql" + `
   query($slug: Slug!) {
     team(slug: $slug) {
       applications(first: 50) {
         nodes { name state teamEnvironment { environment { name } } }
       }
     }
   }
   ` + "```" + `

4. **Get application details with instances**:
   ` + "```graphql" + `
   query($slug: Slug!, $name: String!, $env: [String!]) {
     team(slug: $slug) {
       applications(filter: { name: $name, environments: $env }, first: 1) {
         nodes {
           name state
           instances { nodes { name restarts status { state message } } }
           image { name tag }
         }
       }
     }
   }
   ` + "```" + `

5. **List jobs for a team**:
   ` + "```graphql" + `
   query($slug: Slug!) {
     team(slug: $slug) {
       jobs(first: 50) {
         nodes { name state schedule { expression } teamEnvironment { environment { name } } }
       }
     }
   }
   ` + "```" + `

6. **Get vulnerabilities for a workload**:
   ` + "```graphql" + `
   query($slug: Slug!, $name: String!, $env: [String!]) {
     team(slug: $slug) {
       applications(filter: { name: $name, environments: $env }, first: 1) {
         nodes {
           image {
             vulnerabilitySummary { critical high medium low }
             vulnerabilities(first: 20) { nodes { identifier severity package } }
           }
         }
       }
     }
   }
   ` + "```" + `

7. **Get cost information**:
   ` + "```graphql" + `
   query($slug: Slug!) {
     team(slug: $slug) {
       cost { monthlySummary { sum } }
       environments {
         environment { name }
         cost { daily(from: "2024-01-01", to: "2024-01-31") { sum } }
       }
     }
   }
   ` + "```" + `

8. **Search across resources**:
   ` + "```graphql" + `
   query($query: String!) {
     search(filter: { query: $query }, first: 20) {
       nodes {
         __typename
         ... on Application { name team { slug } }
         ... on Job { name team { slug } }
         ... on Team { slug }
       }
     }
   }
   ` + "```" + `

9. **Get alerts for a team**:
   ` + "```graphql" + `
   query($slug: Slug!) {
     team(slug: $slug) {
       alerts(first: 50) {
         nodes { name state teamEnvironment { environment { name } } }
       }
     }
   }
   ` + "```" + `

10. **Get deployments**:
    ` + "```graphql" + `
    query($slug: Slug!) {
      team(slug: $slug) {
        deployments(first: 20) {
          nodes { createdAt repository commitSha statuses { nodes { state } } }
        }
      }
    }
    ` + "```" + `

### Important Types

- **Slug**: A string identifier for teams (e.g., "my-team")
- **Cursor**: Used for pagination (pass to "after" argument)
- **Date**: Format "YYYY-MM-DD" for cost queries
- **ApplicationState**: RUNNING, NOT_RUNNING, UNKNOWN
- **JobState**: RUNNING, NOT_RUNNING, UNKNOWN
- **Severity**: CRITICAL, HIGH, MEDIUM, LOW, UNASSIGNED (for issues/vulnerabilities)

### Pagination

Most list fields support pagination with:
- ` + "`first: Int`" + ` - Number of items to fetch
- ` + "`after: Cursor`" + ` - Cursor from previous page
- ` + "`pageInfo { hasNextPage endCursor totalCount }`" + ` - Pagination info

### Filtering

Many fields support filters:
- ` + "`filter: { name: String, environments: [String!] }`" + ` - For applications/jobs
- ` + "`filter: { severity: Severity }`" + ` - For issues

### Tips

1. DO NOT query secret-related types/fields (Secret, SecretValue, etc.)
2. Always use ` + "`__typename`" + ` when querying union/interface types (Workload, Issue, etc.)
3. Use fragment spreads for type-specific fields: ` + "`... on Application { ingresses { url } }`" + `
4. Start with schema exploration to discover available fields
5. Use pagination for large result sets (default to first: 50)

### Nais Console URLs

When providing links to the user, use the console URL patterns provided by the ` + "`get_nais_context`" + ` tool.
Call ` + "`get_nais_context`" + ` to get the base URL and all available URL patterns with placeholders.
Replace the placeholders (e.g., ` + "`{team}`" + `, ` + "`{env}`" + `, ` + "`{app}`" + `) with actual values from query results.

**Note**: Do NOT invent or guess URLs. Only use the URL patterns from ` + "`get_nais_context`" + ` with actual data from query results.
`

// GraphQLClient is the interface for executing GraphQL queries.
// This is a subset of the mcp.Client interface focused on GraphQL execution.
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
func (g *GraphQLTools) GetNaisContext(ctx context.Context, input GetNaisContextInput) (GetNaisContextOutput, error) {
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
