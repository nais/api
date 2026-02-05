package mcp

import (
	"io"
	"log/slog"
)

// Transport defines the transport type for the MCP server.
type Transport string

const (
	// TransportStdio uses standard input/output for communication.
	TransportStdio Transport = "stdio"
	// TransportHTTP uses HTTP for communication.
	TransportHTTP Transport = "http"
	// TransportSSE uses Server-Sent Events for communication.
	TransportSSE Transport = "sse"
)

// Config holds the configuration for MCP tools and server.
type Config struct {
	// Client is the GraphQL client for executing queries.
	Client Client

	// SchemaProvider provides access to the GraphQL schema.
	SchemaProvider SchemaProvider

	// TenantName is the tenant identifier (e.g., "nav").
	// Used to construct console URLs: console.<tenant>.cloud.nais.io
	TenantName string

	// Transport specifies the transport type for MCP server (stdio, http, sse).
	// Only used when creating an MCP server.
	Transport Transport

	// ListenAddr is the address to listen on for HTTP/SSE transports.
	// Only used when creating an MCP server with HTTP or SSE transport.
	ListenAddr string

	// Logger is the logger for MCP operations.
	Logger *slog.Logger

	// LogOutput is where logs are written (defaults to stderr).
	LogOutput io.Writer
}

// Option is a functional option for configuring MCP.
type Option func(*Config)

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Transport:  TransportStdio,
		ListenAddr: ":8080",
		Logger:     slog.Default(),
	}
}

// WithClient sets the GraphQL client.
func WithClient(client Client) Option {
	return func(c *Config) {
		c.Client = client
	}
}

// WithSchemaProvider sets the schema provider.
func WithSchemaProvider(provider SchemaProvider) Option {
	return func(c *Config) {
		c.SchemaProvider = provider
	}
}

// WithTenantName sets the tenant name for console URL generation.
func WithTenantName(tenant string) Option {
	return func(c *Config) {
		c.TenantName = tenant
	}
}

// WithTransport sets the transport type for MCP server.
func WithTransport(t Transport) Option {
	return func(c *Config) {
		c.Transport = t
	}
}

// WithListenAddr sets the listen address for HTTP/SSE transports.
func WithListenAddr(addr string) Option {
	return func(c *Config) {
		c.ListenAddr = addr
	}
}

// WithLogger sets the logger.
func WithLogger(logger *slog.Logger) Option {
	return func(c *Config) {
		c.Logger = logger
	}
}

// WithLogOutput sets the log output destination.
func WithLogOutput(w io.Writer) Option {
	return func(c *Config) {
		c.LogOutput = w
	}
}

// ConsoleBaseURL returns the base console URL for the configured tenant.
func (c *Config) ConsoleBaseURL() string {
	if c.TenantName == "" {
		return ""
	}
	return "https://console." + c.TenantName + ".cloud.nais.io"
}

// ConsoleURLPatterns returns the URL patterns for various console pages.
// Placeholders like {team}, {env}, {app} should be replaced with actual values.
func (c *Config) ConsoleURLPatterns() map[string]string {
	return map[string]string{
		"team":                        "/team/{team}",
		"team_applications":           "/team/{team}/applications",
		"team_jobs":                   "/team/{team}/jobs",
		"team_alerts":                 "/team/{team}/alerts",
		"team_issues":                 "/team/{team}/issues",
		"team_vulnerabilities":        "/team/{team}/vulnerabilities",
		"team_cost":                   "/team/{team}/cost",
		"team_deployments":            "/team/{team}/deploy",
		"application":                 "/team/{team}/{env}/app/{app}",
		"application_cost":            "/team/{team}/{env}/app/{app}/cost",
		"application_deploys":         "/team/{team}/{env}/app/{app}/deploys",
		"application_logs":            "/team/{team}/{env}/app/{app}/logs",
		"application_manifest":        "/team/{team}/{env}/app/{app}/manifest",
		"application_vulnerabilities": "/team/{team}/{env}/app/{app}/vulnerabilities",
		"job":                         "/team/{team}/{env}/job/{job}",
		"job_cost":                    "/team/{team}/{env}/job/{job}/cost",
		"job_deploys":                 "/team/{team}/{env}/job/{job}/deploys",
		"job_logs":                    "/team/{team}/{env}/job/{job}/logs",
		"job_manifest":                "/team/{team}/{env}/job/{job}/manifest",
		"job_vulnerabilities":         "/team/{team}/{env}/job/{job}/vulnerabilities",
		"postgres":                    "/team/{team}/{env}/postgres/{name}",
		"opensearch":                  "/team/{team}/{env}/opensearch/{name}",
		"valkey":                      "/team/{team}/{env}/valkey/{name}",
		"bucket":                      "/team/{team}/{env}/bucket/{name}",
		"bigquery":                    "/team/{team}/{env}/bigquery/{name}",
		"kafka":                       "/team/{team}/{env}/kafka/{name}",
	}
}
