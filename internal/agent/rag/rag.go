package rag

import "context"

// DocumentSearcher defines the interface for RAG document retrieval.
// Implementations can use DuckDB, Elasticsearch, or other providers.
type DocumentSearcher interface {
	// Search queries the document store and returns relevant excerpts.
	Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResult, error)

	// Close cleans up any resources held by the searcher.
	Close() error
}

// SearchOptions configures the document search behavior.
type SearchOptions struct {
	// MaxResults is the maximum number of documents to return.
	MaxResults int

	// Filters allows filtering by metadata (e.g., {"source": "docs.nais.io"}).
	Filters map[string]string
}

// SearchResult contains the documents returned from a search.
type SearchResult struct {
	Documents []Document
}

// Document represents a retrieved document excerpt.
type Document struct {
	// Title is the document or section title.
	Title string

	// Content is the relevant text excerpt.
	Content string

	// URL is the source URL for the document.
	URL string

	// Score indicates the relevance score (higher is more relevant).
	Score float64
}
