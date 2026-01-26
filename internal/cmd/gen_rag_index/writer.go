package gen_rag_index

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/sirupsen/logrus"
)

// Writer handles writing chunks and embeddings to a DuckDB file.
type Writer struct {
	db  *sql.DB
	log logrus.FieldLogger
}

// NewWriter creates a new DuckDB writer.
func NewWriter(outputPath string, log logrus.FieldLogger) (*Writer, error) {
	// Ensure the output directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("creating output directory: %w", err)
		}
	}

	// Remove existing file if it exists
	if _, err := os.Stat(outputPath); err == nil {
		if err := os.Remove(outputPath); err != nil {
			return nil, fmt.Errorf("removing existing file: %w", err)
		}
	}

	// Open DuckDB
	db, err := sql.Open("duckdb", outputPath)
	if err != nil {
		return nil, fmt.Errorf("opening DuckDB: %w", err)
	}

	// Create the schema
	if err := createSchema(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	log.WithField("output_path", outputPath).Info("initialized DuckDB writer")

	return &Writer{
		db:  db,
		log: log,
	}, nil
}

// createSchema creates the rag_documents table.
func createSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE rag_documents (
			title TEXT NOT NULL,
			url TEXT NOT NULL,
			content TEXT NOT NULL,
			embedding BLOB NOT NULL
		)
	`)
	return err
}

// ChunkWithEmbedding represents a chunk with its embedding vector.
type ChunkWithEmbedding struct {
	Chunk     Chunk
	Embedding []float32
}

// WriteChunks writes chunks with embeddings to the database.
func (w *Writer) WriteChunks(ctx context.Context, chunks []ChunkWithEmbedding) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO rag_documents (title, url, content, embedding)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("preparing statement: %w", err)
	}
	defer stmt.Close()

	for _, chunk := range chunks {
		embeddingBytes := encodeEmbedding(chunk.Embedding)
		_, err := stmt.ExecContext(ctx, chunk.Chunk.Title, chunk.Chunk.URL, chunk.Chunk.Content, embeddingBytes)
		if err != nil {
			return fmt.Errorf("inserting chunk %q: %w", chunk.Chunk.Title, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	w.log.WithField("count", len(chunks)).Info("wrote chunks to database")
	return nil
}

// CreateIndex creates an index on the url column for faster lookups.
func (w *Writer) CreateIndex(ctx context.Context) error {
	_, err := w.db.ExecContext(ctx, `CREATE INDEX idx_url ON rag_documents(url)`)
	if err != nil {
		return fmt.Errorf("creating index: %w", err)
	}
	w.log.Info("created index on url column")
	return nil
}

// GetStats returns statistics about the written data.
func (w *Writer) GetStats(ctx context.Context) (Stats, error) {
	var stats Stats

	row := w.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM rag_documents`)
	if err := row.Scan(&stats.TotalChunks); err != nil {
		return stats, fmt.Errorf("counting chunks: %w", err)
	}

	row = w.db.QueryRowContext(ctx, `SELECT COUNT(DISTINCT url) FROM rag_documents`)
	if err := row.Scan(&stats.UniqueURLs); err != nil {
		return stats, fmt.Errorf("counting unique URLs: %w", err)
	}

	row = w.db.QueryRowContext(ctx, `SELECT AVG(LENGTH(content)) FROM rag_documents`)
	if err := row.Scan(&stats.AvgContentLength); err != nil {
		return stats, fmt.Errorf("calculating avg content length: %w", err)
	}

	return stats, nil
}

// Stats contains statistics about the generated index.
type Stats struct {
	TotalChunks      int
	UniqueURLs       int
	AvgContentLength float64
}

// Close closes the database connection.
func (w *Writer) Close() error {
	return w.db.Close()
}

// encodeEmbedding converts a float32 slice to bytes for storage.
// Uses little-endian format to match the searcher's parseEmbedding function.
func encodeEmbedding(embedding []float32) []byte {
	buf := make([]byte, len(embedding)*4)
	for i, v := range embedding {
		bits := math.Float32bits(v)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	return buf
}
