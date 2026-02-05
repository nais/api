package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/nais/api/pkg/mcp/tools"
)

const (
	serverName    = "nais-mcp"
	serverVersion = "0.1.0"
)

// Server wraps the MCP server with Nais-specific configuration.
// Use this for the CLI use case where MCP protocol is required.
type Server struct {
	mcpServer *server.MCPServer
	config    *Config
	registry  *tools.Registry
}

// NewServer creates a new MCP server with the given options.
// Use this for CLI integration with AI assistants that support MCP.
func NewServer(opts ...Option) (*Server, error) {
	config := DefaultConfig()
	for _, opt := range opts {
		opt(config)
	}

	if config.Client == nil {
		return nil, ErrClientRequired
	}
	if config.SchemaProvider == nil {
		return nil, ErrSchemaProviderRequired
	}

	// Create the MCP server with capabilities
	mcpServer := server.NewMCPServer(
		serverName,
		serverVersion,
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, false), // resources enabled, no subscriptions
		server.WithRecovery(),
	)

	// Create the unified tool registry
	registry := tools.NewRegistry(tools.RegistryConfig{
		Client:             &clientAdapter{config.Client},
		Schema:             config.SchemaProvider.GetSchema(),
		ConsoleBaseURL:     config.ConsoleBaseURL(),
		ConsoleURLPatterns: config.ConsoleURLPatterns(),
		Logger:             config.Logger,
	})

	s := &Server{
		mcpServer: mcpServer,
		config:    config,
		registry:  registry,
	}

	// Register tools from the registry
	s.registerTools()

	// Register resources
	s.registerResources()

	return s, nil
}

// Serve starts the MCP server with the configured transport.
func (s *Server) Serve(ctx context.Context) error {
	switch s.config.Transport {
	case TransportStdio:
		return s.serveStdio()
	case TransportHTTP:
		return s.serveHTTP(ctx)
	case TransportSSE:
		return s.serveSSE(ctx)
	default:
		return fmt.Errorf("unknown transport: %s", s.config.Transport)
	}
}

// serveStdio starts the server with STDIO transport.
func (s *Server) serveStdio() error {
	return server.ServeStdio(s.mcpServer)
}

// serveHTTP starts the server with HTTP transport.
func (s *Server) serveHTTP(ctx context.Context) error {
	httpServer := server.NewStreamableHTTPServer(s.mcpServer,
		server.WithStateLess(true),
	)
	s.config.Logger.Info("Starting HTTP server", "address", s.config.ListenAddr)
	return httpServer.Start(s.config.ListenAddr)
}

// serveSSE starts the server with SSE transport.
func (s *Server) serveSSE(ctx context.Context) error {
	sseServer := server.NewSSEServer(s.mcpServer)
	s.config.Logger.Info("Starting SSE server", "address", s.config.ListenAddr)
	return sseServer.Start(s.config.ListenAddr)
}

// MCPServer returns the underlying MCP server.
// This is useful for testing or advanced configuration.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpServer
}

// Registry returns the underlying tool registry.
// This is useful for accessing tool definitions or advanced configuration.
func (s *Server) Registry() *tools.Registry {
	return s.registry
}

// registerTools registers all MCP tools from the registry with the server.
func (s *Server) registerTools() {
	// Get all tools from the registry and add them to the MCP server
	serverTools := s.registry.GetMCPServerTools()
	for _, st := range serverTools {
		s.mcpServer.AddTool(st.Tool, st.Handler)
	}
}

// registerResources registers MCP resources with the server.
func (s *Server) registerResources() {
	// GraphQL Schema resource
	schemaResource := mcp.NewResource(
		"nais://schema",
		"GraphQL Schema",
		mcp.WithResourceDescription("The Nais GraphQL API schema"),
		mcp.WithMIMEType("text/plain"),
	)

	s.mcpServer.AddResource(schemaResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.config.Logger.Debug("Reading schema resource")

		schema := s.config.SchemaProvider.GetSchemaSDL()

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     schema,
			},
		}, nil
	})

	// Best practices resource
	bestPracticesResource := mcp.NewResource(
		"nais://api-best-practices",
		"API Best Practices",
		mcp.WithResourceDescription("Best practices and guidelines for using the Nais API"),
		mcp.WithMIMEType("text/markdown"),
	)

	s.mcpServer.AddResource(bestPracticesResource, func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		s.config.Logger.Debug("Reading best practices resource")

		content := getBestPracticesContent()

		return []mcp.ResourceContents{
			mcp.TextResourceContents{
				URI:      req.Params.URI,
				MIMEType: "text/markdown",
				Text:     content,
			},
		}, nil
	})
}

// getBestPracticesContent returns the best practices documentation.
func getBestPracticesContent() string {
	return `# Nais API Best Practices

## Pagination

When querying paginated connections (lists), always use reasonable page sizes:

- **Recommended page size**: 20-50 items
- **Maximum recommended**: 100 items
- **Never use**: 1000 or unlimited queries

### Example - Correct pagination:

` + "```graphql" + `
query TeamWorkloads($slug: Slug!, $first: Int!, $after: Cursor) {
  team(slug: $slug) {
    workloads(first: $first, after: $after) {
      nodes {
        name
        image { name tag }
      }
      pageInfo {
        hasNextPage
        endCursor
      }
    }
  }
}
` + "```" + `

Call with ` + "`first: 50`" + ` and use ` + "`endCursor`" + ` to fetch subsequent pages.

### Why pagination matters:

1. **Performance**: Large queries are slow and may timeout
2. **Resource usage**: Reduces load on the API server
3. **Reliability**: Smaller pages are less likely to fail

## Query Optimization

### Request only needed fields

Instead of requesting all fields, specify only what you need:

` + "```graphql" + `
# Good - only request needed fields
query {
  team(slug: "my-team") {
    applications(first: 50) {
      nodes {
        name
        state
      }
    }
  }
}
` + "```" + `

### Use filters when available

Most connections support filtering to reduce result size:

` + "```graphql" + `
query {
  team(slug: "my-team") {
    applications(first: 50, filter: { environments: ["prod"] }) {
      nodes { name state }
    }
  }
}
` + "```" + `

## Common Patterns

### Iterating over all teams

` + "```graphql" + `
query MyTeams {
  me {
    ... on User {
      teams(first: 50) {
        nodes {
          team { slug }
        }
        pageInfo { hasNextPage endCursor }
      }
    }
  }
}
` + "```" + `

### Getting workloads across environments

` + "```graphql" + `
query TeamWorkloads($slug: Slug!) {
  team(slug: $slug) {
    workloads(first: 50) {
      nodes {
        __typename
        name
        teamEnvironment {
          environment { name }
        }
        image { name tag }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}
` + "```" + `
`
}
