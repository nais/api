package gen_rag_index

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/nais/api/internal/agent/rag/duckdb"
	"github.com/nais/api/internal/logger"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
)

const (
	exitCodeSuccess = iota
	exitCodeConfigError
	exitCodeLoggerError
	exitCodeRunError
)

// Run is the main entry point for the RAG index generator.
func Run(ctx context.Context) {
	log, err := logger.New("text", "INFO")
	if err != nil {
		fmt.Printf("logger error: %s\n", err)
		os.Exit(exitCodeLoggerError)
	}
	loadEnvFile(log)

	cfg, err := NewConfig(ctx, envconfig.OsLookuper())
	if err != nil {
		log.WithError(err).Error("configuration error")
		os.Exit(exitCodeConfigError)
	}

	if err := run(ctx, cfg, log); err != nil {
		log.WithError(err).Error("fatal error")
		os.Exit(exitCodeRunError)
	}

	os.Exit(exitCodeSuccess)
}

func run(ctx context.Context, cfg *Config, log logrus.FieldLogger) error {
	startTime := time.Now()

	log.WithFields(logrus.Fields{
		"tenant":       cfg.TenantName,
		"output":       cfg.OutputPath,
		"chunk_size":   cfg.ChunkMaxChars,
		"search_index": cfg.SearchIndexURL(),
	}).Info("starting RAG index generation")

	// Step 1: Download search index
	log.Info("downloading search index...")
	index, err := DownloadSearchIndex(ctx, cfg.SearchIndexURL())
	if err != nil {
		return fmt.Errorf("downloading search index: %w", err)
	}
	log.WithField("docs", len(index.Docs)).Info("downloaded search index")

	// Step 2: Process into pages
	log.Info("processing documents into pages...")
	pages := ProcessSearchIndex(index, cfg.DocsBaseURL())
	log.WithField("pages", len(pages)).Info("processed pages")

	// Step 3: Chunk pages
	log.Info("chunking pages...")
	chunks := ChunkPages(pages, cfg.ChunkMaxChars)
	log.WithField("chunks", len(chunks)).Info("created chunks")

	// Step 4: Initialize embedding client
	log.Info("initializing embedding client...")
	embeddingClient, err := duckdb.NewVertexAIEmbeddingClient(ctx, duckdb.EmbeddingConfig{
		ProjectID: cfg.VertexAI.ProjectID,
		Location:  cfg.VertexAI.Location,
		ModelName: cfg.VertexAI.EmbeddingModel,
	}, log.WithField("component", "embedding-client"))
	if err != nil {
		return fmt.Errorf("creating embedding client: %w", err)
	}
	defer embeddingClient.Close()

	// Step 5: Generate embeddings
	log.Info("generating embeddings...")
	chunksWithEmbeddings, err := generateEmbeddings(ctx, embeddingClient, chunks, log)
	if err != nil {
		return fmt.Errorf("generating embeddings: %w", err)
	}
	log.WithField("embedded", len(chunksWithEmbeddings)).Info("generated embeddings")

	// Step 6: Write to DuckDB
	log.Info("writing to DuckDB...")
	writer, err := NewWriter(cfg.OutputPath, log.WithField("component", "writer"))
	if err != nil {
		return fmt.Errorf("creating writer: %w", err)
	}
	defer writer.Close()

	if err := writer.WriteChunks(ctx, chunksWithEmbeddings); err != nil {
		return fmt.Errorf("writing chunks: %w", err)
	}

	if err := writer.CreateIndex(ctx); err != nil {
		return fmt.Errorf("creating index: %w", err)
	}

	// Step 7: Print stats
	stats, err := writer.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("getting stats: %w", err)
	}

	duration := time.Since(startTime)
	log.WithFields(logrus.Fields{
		"total_chunks":       stats.TotalChunks,
		"unique_urls":        stats.UniqueURLs,
		"avg_content_length": fmt.Sprintf("%.0f", stats.AvgContentLength),
		"duration":           duration.Round(time.Second).String(),
		"output":             cfg.OutputPath,
	}).Info("RAG index generation complete")

	return nil
}

// generateEmbeddings generates embeddings for all chunks using batch API calls.
// The Vertex AI API supports up to 250 texts per batch, but we use 100 for safety.
func generateEmbeddings(ctx context.Context, client *duckdb.VertexAIEmbeddingClient, chunks []Chunk, log logrus.FieldLogger) ([]ChunkWithEmbedding, error) {
	const batchSize = 100

	result := make([]ChunkWithEmbedding, 0, len(chunks))
	totalBatches := (len(chunks) + batchSize - 1) / batchSize

	for batchNum := 0; batchNum < totalBatches; batchNum++ {
		start := batchNum * batchSize
		end := start + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[start:end]

		// Extract content for embedding
		texts := make([]string, len(batch))
		for i, chunk := range batch {
			texts[i] = chunk.Content
		}

		// Generate embeddings for the batch
		embeddings, err := client.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("embedding batch %d (chunks %d-%d): %w", batchNum+1, start, end-1, err)
		}

		// Combine chunks with their embeddings
		for i, chunk := range batch {
			result = append(result, ChunkWithEmbedding{
				Chunk:     chunk,
				Embedding: embeddings[i],
			})
		}

		// Progress logging
		log.WithFields(logrus.Fields{
			"batch":   fmt.Sprintf("%d/%d", batchNum+1, totalBatches),
			"chunks":  fmt.Sprintf("%d/%d", end, len(chunks)),
			"percent": fmt.Sprintf("%.1f%%", float64(end)/float64(len(chunks))*100),
		}).Info("embedding progress")
	}

	return result, nil
}

// loadEnvFile will load a .env file if it exists. This is useful for local development.
func loadEnvFile(log logrus.FieldLogger) error {
	if _, err := os.Stat(".env"); errors.Is(err, os.ErrNotExist) {
		log.Infof("no .env file found")
		return nil
	}

	if err := godotenv.Load(".env"); err != nil {
		return err
	}

	log.Infof("loaded .env file")
	return nil
}
