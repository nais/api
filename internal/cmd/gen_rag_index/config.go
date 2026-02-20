package gen_rag_index

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

// Config holds configuration for the RAG index generator.
type Config struct {
	// TenantName is the tenant name used to construct the docs URL.
	// The search index is fetched from: https://docs.<TenantName>.cloud.nais.io/search/search_index.json
	TenantName string `env:"TENANT,required"`

	// OutputPath is the path where the DuckDB file will be written.
	OutputPath string `env:"OUTPUT_PATH,default=./data/rag_index.duckdb"`

	// ChunkMaxChars is the maximum number of characters per chunk.
	ChunkMaxChars int `env:"CHUNK_MAX_CHARS,default=1500"`

	// VertexAI contains Vertex AI specific configuration for embeddings.
	VertexAI VertexAIConfig
}

// VertexAIConfig contains Vertex AI specific configuration.
type VertexAIConfig struct {
	// ProjectID is the GCP project hosting Vertex AI resources.
	ProjectID string `env:"AGENT_VERTEX_AI_PROJECT_ID,required"`

	// Location is the region for Vertex AI (must be in EU, e.g., "europe-west1").
	Location string `env:"AGENT_VERTEX_AI_LOCATION,default=europe-west1"`

	// EmbeddingModel is the model to use for generating embeddings.
	EmbeddingModel string `env:"AGENT_VERTEX_AI_EMBEDDING_MODEL,default=gemini-embedding-001"`
}

// NewConfig creates a new configuration instance from environment variables.
func NewConfig(ctx context.Context, lookuper envconfig.Lookuper) (*Config, error) {
	cfg := &Config{}
	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	})
	if err != nil {
		return nil, fmt.Errorf("processing config: %w", err)
	}

	return cfg, nil
}

// DocsBaseURL returns the base URL for the documentation site.
func (c *Config) DocsBaseURL() string {
	return fmt.Sprintf("https://docs.%s.cloud.nais.io", c.TenantName)
}

// SearchIndexURL returns the URL for the search index JSON file.
func (c *Config) SearchIndexURL() string {
	return fmt.Sprintf("%s/search/search_index.json", c.DocsBaseURL())
}
