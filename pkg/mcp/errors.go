package mcp

import "errors"

// Sentinel errors for the MCP package.
var (
	// ErrClientRequired is returned when a Client is not provided.
	ErrClientRequired = errors.New("mcp: client is required")

	// ErrSchemaProviderRequired is returned when a SchemaProvider is not provided.
	ErrSchemaProviderRequired = errors.New("mcp: schema provider is required")

	// ErrTenantNameRequired is returned when a TenantName is not provided.
	ErrTenantNameRequired = errors.New("mcp: tenant name is required")

	// ErrUnknownTool is returned when an unknown tool is requested.
	ErrUnknownTool = errors.New("mcp: unknown tool")

	// ErrQueryNotAllowed is returned when a query operation is not permitted.
	ErrQueryNotAllowed = errors.New("mcp: only query operations are allowed")

	// ErrForbiddenType is returned when a query accesses a forbidden type.
	ErrForbiddenType = errors.New("mcp: query accesses forbidden type containing sensitive data")

	// ErrQueryDepthExceeded is returned when a query exceeds the maximum depth.
	ErrQueryDepthExceeded = errors.New("mcp: query depth exceeds maximum allowed")

	// ErrInvalidVariables is returned when query variables are invalid JSON.
	ErrInvalidVariables = errors.New("mcp: invalid variables JSON")

	// ErrRateLimitExceeded is returned when the rate limit is exceeded.
	ErrRateLimitExceeded = errors.New("mcp: rate limit exceeded")
)
