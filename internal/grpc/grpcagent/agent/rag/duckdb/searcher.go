// Package duckdb implements the rag.DocumentSearcher interface using a local DuckDB file.
package duckdb

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"sort"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/nais/api/internal/grpc/grpcagent/agent/rag"
	"github.com/sirupsen/logrus"
)

// Config holds configuration for the DuckDB searcher.
type Config struct {
	// DBPath is the path to the DuckDB file.
	DBPath string

	// EmbeddingClient is used to embed queries at runtime.
	EmbeddingClient EmbeddingClient
}

// EmbeddingClient generates embeddings for text.
type EmbeddingClient interface {
	// Embed returns the embedding vector for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Searcher implements rag.DocumentSearcher using a local DuckDB file.
type Searcher struct {
	db              *sql.DB
	embeddingClient EmbeddingClient
	log             logrus.FieldLogger
}

// NewSearcher creates a new DuckDB-based searcher.
func NewSearcher(cfg Config, log logrus.FieldLogger) (*Searcher, error) {
	// Open DuckDB in read-only mode
	db, err := sql.Open("duckdb", cfg.DBPath+"?access_mode=read_only")
	if err != nil {
		return nil, fmt.Errorf("failed to open DuckDB: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping DuckDB: %w", err)
	}

	log.WithField("db_path", cfg.DBPath).Info("initialized DuckDB searcher")

	return &Searcher{
		db:              db,
		embeddingClient: cfg.EmbeddingClient,
		log:             log,
	}, nil
}

// Search queries the document store and returns relevant excerpts.
func (s *Searcher) Search(ctx context.Context, query string, opts *rag.SearchOptions) (*rag.SearchResult, error) {
	maxResults := 5
	if opts != nil && opts.MaxResults > 0 {
		maxResults = opts.MaxResults
	}

	// Embed the query
	queryEmbedding, err := s.embeddingClient.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	// Fetch all documents with embeddings
	// For small indices (~15MB), loading all into memory is acceptable
	// Future optimization: use DuckDB vector extensions for ANN search
	rows, err := s.db.QueryContext(ctx, `
		SELECT title, url, content, embedding
		FROM rag_documents
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query documents: %w", err)
	}
	defer rows.Close()

	type scoredDoc struct {
		doc   rag.Document
		score float64
	}

	var docs []scoredDoc

	for rows.Next() {
		var title, url, content string
		var embeddingBytes []byte

		if err := rows.Scan(&title, &url, &content, &embeddingBytes); err != nil {
			s.log.WithError(err).Warn("failed to scan row")
			continue
		}

		// Parse embedding from stored bytes
		docEmbedding, err := parseEmbedding(embeddingBytes)
		if err != nil {
			s.log.WithError(err).Warn("failed to parse embedding")
			continue
		}

		// Calculate cosine similarity
		score := cosineSimilarity(queryEmbedding, docEmbedding)

		docs = append(docs, scoredDoc{
			doc: rag.Document{
				Title:   title,
				URL:     url,
				Content: content,
				Score:   score,
			},
			score: score,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	// Sort by score descending
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].score > docs[j].score
	})

	// Take top-k results
	result := &rag.SearchResult{
		Documents: make([]rag.Document, 0, maxResults),
	}

	for i := 0; i < len(docs) && i < maxResults; i++ {
		result.Documents = append(result.Documents, docs[i].doc)
	}

	s.log.WithFields(logrus.Fields{
		"query":       query,
		"total_docs":  len(docs),
		"returned":    len(result.Documents),
		"max_results": maxResults,
	}).Debug("search completed")

	return result, nil
}

// Close cleans up resources.
func (s *Searcher) Close() error {
	return s.db.Close()
}

// parseEmbedding converts stored bytes to a float32 slice.
// The embedding is stored as a binary blob of float32 values.
func parseEmbedding(data []byte) ([]float32, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty embedding data")
	}

	// Each float32 is 4 bytes
	if len(data)%4 != 0 {
		return nil, fmt.Errorf("invalid embedding data length: %d", len(data))
	}

	numFloats := len(data) / 4
	embedding := make([]float32, numFloats)

	for i := 0; i < numFloats; i++ {
		offset := i * 4
		bits := uint32(data[offset]) |
			uint32(data[offset+1])<<8 |
			uint32(data[offset+2])<<16 |
			uint32(data[offset+3])<<24
		embedding[i] = math.Float32frombits(bits)
	}

	return embedding, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors.
func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64

	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
