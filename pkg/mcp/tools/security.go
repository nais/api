package tools

import (
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
)

const maxQueryDepth = 15

// forbiddenTypes are GraphQL types that contain sensitive data and should not be accessible via MCP queries.
// These types and their fields expose secret values that should not be returned to LLMs.
var forbiddenTypes = map[string]bool{
	"Secret":                           true, // The Secret type contains secret values
	"SecretValue":                      true, // SecretValue contains the actual secret data
	"SecretConnection":                 true, // Connection type that returns Secret nodes
	"SecretEdge":                       true, // Edge type that wraps Secret
	"DeploymentKey":                    true, // Contains the actual deployment key
	"CreateServiceAccountTokenPayload": true, // Contains the service account token secret
	"ServiceAccountToken":              true, // Service account token metadata (but secret field is blocked separately)
	"ServiceAccountTokenConnection":    true, // Connection type that returns ServiceAccountToken nodes
	"ServiceAccountTokenEdge":          true, // Edge type that wraps ServiceAccountToken
}

// QueryValidationResult contains the result of validating a GraphQL query.
type QueryValidationResult struct {
	Valid         bool
	Error         string
	OperationType string
	OperationName string
	Depth         int
}

// checkForSecrets recursively checks if a selection set accesses any forbidden secret-related types.
// It validates against the GraphQL schema to ensure queries don't access Secret or SecretValue types.
func checkForSecrets(selectionSet ast.SelectionSet, schema *ast.Schema) (bool, string) {
	for _, selection := range selectionSet {
		switch sel := selection.(type) {
		case *ast.Field:
			// Check if this field's definition exists in the schema
			if sel.Definition != nil {
				// Check if the field returns a forbidden type
				typeName := getBaseTypeName(sel.Definition.Type)
				if forbiddenTypes[typeName] {
					return true, fmt.Sprintf("MCP security policy: field '%s' returns type '%s' which contains sensitive data that cannot be accessed via this interface. Use the Nais Console or CLI to manage secrets directly.", sel.Name, typeName)
				}
			}

			// Recursively check nested selections
			if len(sel.SelectionSet) > 0 {
				if found, reason := checkForSecrets(sel.SelectionSet, schema); found {
					return true, reason
				}
			}
		case *ast.InlineFragment:
			// Check if the inline fragment is on a forbidden type
			if sel.TypeCondition != "" && forbiddenTypes[sel.TypeCondition] {
				return true, fmt.Sprintf("MCP security policy: inline fragment on type '%s' which contains sensitive data that cannot be accessed via this interface", sel.TypeCondition)
			}
			// Check inline fragments recursively
			if found, reason := checkForSecrets(sel.SelectionSet, schema); found {
				return true, reason
			}
		case *ast.FragmentSpread:
			// Fragment spreads would need fragment definitions to be fully validated
			// For now, we flag any fragment that has "secret" in its name as a heuristic
			if strings.Contains(strings.ToLower(sel.Name), "secret") {
				return true, fmt.Sprintf("MCP security policy: fragment '%s' may access sensitive data that cannot be accessed via this interface", sel.Name)
			}
		}
	}
	return false, ""
}

// getBaseTypeName extracts the base type name from a GraphQL type, removing list and non-null wrappers.
func getBaseTypeName(t *ast.Type) string {
	if t.Elem != nil {
		return getBaseTypeName(t.Elem)
	}
	return t.Name()
}

// calculateQueryDepth calculates the maximum depth of a selection set.
func calculateQueryDepth(selectionSet ast.SelectionSet, currentDepth int) int {
	if len(selectionSet) == 0 {
		return currentDepth
	}

	maxDepth := currentDepth
	for _, selection := range selectionSet {
		var childDepth int
		switch sel := selection.(type) {
		case *ast.Field:
			childDepth = calculateQueryDepth(sel.SelectionSet, currentDepth+1)
		case *ast.InlineFragment:
			childDepth = calculateQueryDepth(sel.SelectionSet, currentDepth)
		case *ast.FragmentSpread:
			// Fragment spreads would need to be resolved against fragment definitions
			// For simplicity, we count them as +1 depth
			childDepth = currentDepth + 1
		}
		if childDepth > maxDepth {
			maxDepth = childDepth
		}
	}
	return maxDepth
}

// IsForbiddenType checks if a type name is in the forbidden types list.
func IsForbiddenType(typeName string) bool {
	return forbiddenTypes[typeName]
}
