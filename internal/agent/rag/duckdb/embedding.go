package duckdb

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

// EmbeddingConfig holds configuration for the Vertex AI embedding client.
type EmbeddingConfig struct {
	// ProjectID is the GCP project ID.
	ProjectID string

	// Location is the GCP region (must be in EU, e.g., "europe-west1").
	Location string

	// ModelName is the embedding model to use (e.g., "gemini-embedding-001").
	ModelName string
}

// VertexAIEmbeddingClient implements EmbeddingClient using Vertex AI.
type VertexAIEmbeddingClient struct {
	client *genai.Client
	model  string
	log    logrus.FieldLogger
}

// NewVertexAIEmbeddingClient creates a new Vertex AI embedding client.
func NewVertexAIEmbeddingClient(ctx context.Context, cfg EmbeddingConfig, log logrus.FieldLogger) (*VertexAIEmbeddingClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  cfg.ProjectID,
		Location: cfg.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	log.WithFields(logrus.Fields{
		"project":  cfg.ProjectID,
		"location": cfg.Location,
		"model":    cfg.ModelName,
	}).Info("initialized Vertex AI embedding client")

	return &VertexAIEmbeddingClient{
		client: client,
		model:  cfg.ModelName,
		log:    log,
	}, nil
}

// Embed returns the embedding vector for the given text.
func (c *VertexAIEmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch returns embedding vectors for multiple texts in a single API call.
// This is much more efficient than calling Embed multiple times.
// The Vertex AI API supports up to 250 texts per batch request.
func (c *VertexAIEmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = &genai.Content{Parts: []*genai.Part{{Text: text}}}
	}

	result, err := c.client.Models.EmbedContent(ctx, c.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to embed texts: %w", err)
	}

	if result == nil || len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	embeddings := make([][]float32, len(texts))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}

	return embeddings, nil
}

// Close cleans up resources.
func (c *VertexAIEmbeddingClient) Close() error {
	// The genai client doesn't have a Close method
	return nil
}
