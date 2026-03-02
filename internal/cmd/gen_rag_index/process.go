package gen_rag_index

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strings"
)

// SearchIndex represents the MkDocs search index structure.
type SearchIndex struct {
	Config SearchConfig `json:"config"`
	Docs   []SearchDoc  `json:"docs"`
}

// SearchConfig contains search configuration from MkDocs.
type SearchConfig struct {
	Lang      []string               `json:"lang"`
	Separator string                 `json:"separator"`
	Pipeline  []string               `json:"pipeline"`
	Fields    map[string]FieldConfig `json:"fields"`
}

// FieldConfig contains field-specific search configuration.
type FieldConfig struct {
	Boost float64 `json:"boost"`
}

// SearchDoc represents a single document in the search index.
type SearchDoc struct {
	Location string   `json:"location"`
	Title    string   `json:"title"`
	Text     string   `json:"text"`
	Tags     []string `json:"tags,omitempty"`
}

// Page represents a processed documentation page.
type Page struct {
	// Path is the page path (location without fragment).
	Path string

	// Title is the page title.
	Title string

	// Tags are the tags associated with this page.
	Tags []string

	// Sections contains the page sections.
	Sections []Section

	// URL is the full URL to the page.
	URL string
}

// Section represents a section within a page.
type Section struct {
	Title string
	Text  string
}

// Chunk represents a chunk of content ready for embedding.
type Chunk struct {
	// Title is the page title (included in all chunks from the same page).
	Title string

	// URL is the full URL to the page.
	URL string

	// Content is the formatted content for embedding.
	Content string
}

// DownloadSearchIndex downloads and parses the search index from the docs site.
func DownloadSearchIndex(ctx context.Context, url string) (*SearchIndex, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching search index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var index SearchIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("parsing search index: %w", err)
	}

	return &index, nil
}

// ProcessSearchIndex processes the search index into pages, filtering out unwanted content.
func ProcessSearchIndex(index *SearchIndex, baseURL string) []Page {
	// Group documents by page (location without fragment)
	pageMap := make(map[string]*Page)
	pageOrder := make([]string, 0)

	for _, doc := range index.Docs {
		// Skip empty documents
		if doc.Text == "" && doc.Title == "" {
			continue
		}

		// Extract page path (without fragment)
		pagePath := getPagePath(doc.Location)

		// Skip tag index pages (they're just lists of links)
		if isTagIndexPage(pagePath, doc.Location) {
			continue
		}

		// Get or create page
		page, exists := pageMap[pagePath]
		if !exists {
			page = &Page{
				Path:     pagePath,
				URL:      buildURL(baseURL, pagePath),
				Sections: make([]Section, 0),
			}
			pageMap[pagePath] = page
			pageOrder = append(pageOrder, pagePath)
		}

		// Set page title from the root document (no fragment)
		if !strings.Contains(doc.Location, "#") || page.Title == "" {
			if doc.Title != "" {
				page.Title = doc.Title
			}
		}

		// Collect tags
		if len(doc.Tags) > 0 {
			page.Tags = mergeTags(page.Tags, doc.Tags)
		}

		// Add section content
		if doc.Text != "" {
			page.Sections = append(page.Sections, Section{
				Title: doc.Title,
				Text:  stripHTML(doc.Text),
			})
		}
	}

	// Convert map to slice in original order
	pages := make([]Page, 0, len(pageOrder))
	for _, path := range pageOrder {
		if page := pageMap[path]; page != nil && len(page.Sections) > 0 {
			pages = append(pages, *page)
		}
	}

	return pages
}

// ChunkPages converts pages into chunks suitable for embedding.
func ChunkPages(pages []Page, maxChars int) []Chunk {
	chunks := make([]Chunk, 0)

	for _, page := range pages {
		pageChunks := chunkPage(page, maxChars)
		chunks = append(chunks, pageChunks...)
	}

	return chunks
}

// chunkPage splits a single page into chunks.
func chunkPage(page Page, maxChars int) []Chunk {
	chunks := make([]Chunk, 0)

	// Build the header that will be included in each chunk
	header := buildChunkHeader(page.Title, page.Tags)
	headerLen := len(header)

	// Calculate available space for content
	availableChars := maxChars - headerLen
	if availableChars < 100 {
		// If header is too long, use a smaller header
		header = fmt.Sprintf("Title: %s\n\n", page.Title)
		headerLen = len(header)
		availableChars = maxChars - headerLen
	}

	// Combine all section content
	var contentBuilder strings.Builder
	for i, section := range page.Sections {
		if i > 0 {
			contentBuilder.WriteString("\n\n")
		}
		if section.Title != "" && section.Title != page.Title {
			contentBuilder.WriteString("## ")
			contentBuilder.WriteString(section.Title)
			contentBuilder.WriteString("\n")
		}
		contentBuilder.WriteString(section.Text)
	}
	fullContent := contentBuilder.String()

	// If content fits in one chunk, return it
	if len(fullContent) <= availableChars {
		chunks = append(chunks, Chunk{
			Title:   page.Title,
			URL:     page.URL,
			Content: header + fullContent,
		})
		return chunks
	}

	// Split content into multiple chunks
	contentChunks := splitContent(fullContent, availableChars)
	for _, content := range contentChunks {
		chunks = append(chunks, Chunk{
			Title:   page.Title,
			URL:     page.URL,
			Content: header + content,
		})
	}

	return chunks
}

// buildChunkHeader creates the header for a chunk including title and tags.
func buildChunkHeader(title string, tags []string) string {
	var builder strings.Builder
	builder.WriteString("Title: ")
	builder.WriteString(title)
	builder.WriteString("\n")

	if len(tags) > 0 {
		builder.WriteString("Tags: ")
		builder.WriteString(strings.Join(tags, ", "))
		builder.WriteString("\n")
	}

	builder.WriteString("\n")
	return builder.String()
}

// splitContent splits content into chunks at natural boundaries.
func splitContent(content string, maxChars int) []string {
	if len(content) <= maxChars {
		return []string{content}
	}

	chunks := make([]string, 0)
	remaining := content

	for len(remaining) > 0 {
		if len(remaining) <= maxChars {
			chunks = append(chunks, strings.TrimSpace(remaining))
			break
		}

		// Find a good split point
		splitPoint := findSplitPoint(remaining, maxChars)
		chunk := strings.TrimSpace(remaining[:splitPoint])
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		remaining = remaining[splitPoint:]
	}

	return chunks
}

// findSplitPoint finds a good point to split content, preferring natural boundaries.
func findSplitPoint(content string, maxChars int) int {
	if len(content) <= maxChars {
		return len(content)
	}

	// Look for section boundaries (## headers) first
	sectionPattern := regexp.MustCompile(`\n## `)
	if loc := sectionPattern.FindStringIndex(content[maxChars/2 : maxChars]); loc != nil {
		return maxChars/2 + loc[0]
	}

	// Look for paragraph boundaries
	for i := maxChars - 1; i > maxChars/2; i-- {
		if i < len(content) && content[i] == '\n' && i+1 < len(content) && content[i+1] == '\n' {
			return i + 1
		}
	}

	// Look for sentence boundaries
	for i := maxChars - 1; i > maxChars/2; i-- {
		if i < len(content) && (content[i] == '.' || content[i] == '!' || content[i] == '?') {
			if i+1 < len(content) && (content[i+1] == ' ' || content[i+1] == '\n') {
				return i + 1
			}
		}
	}

	// Look for word boundaries
	for i := maxChars - 1; i > maxChars/2; i-- {
		if i < len(content) && content[i] == ' ' {
			return i + 1
		}
	}

	// Fall back to hard cut
	return maxChars
}

// getPagePath extracts the page path from a location (removes fragment).
func getPagePath(location string) string {
	if idx := strings.Index(location, "#"); idx != -1 {
		return location[:idx]
	}
	return location
}

// isTagIndexPage checks if a document is a tag index page that should be skipped.
func isTagIndexPage(pagePath, location string) bool {
	// Skip the main tags page and individual tag pages
	if pagePath == "tags/" || strings.HasPrefix(pagePath, "tags/") {
		return true
	}
	// Skip tag fragments on other pages
	if strings.Contains(location, "#tag:") {
		return true
	}
	return false
}

// buildURL constructs the full URL for a page.
func buildURL(baseURL, path string) string {
	if path == "" {
		return baseURL + "/"
	}
	return baseURL + "/" + path
}

// mergeTags merges two tag slices, removing duplicates.
func mergeTags(existing, new []string) []string {
	seen := make(map[string]bool)
	for _, t := range existing {
		seen[t] = true
	}

	result := append([]string{}, existing...)
	for _, t := range new {
		if !seen[t] {
			seen[t] = true
			result = append(result, t)
		}
	}
	return result
}

// HTML tag patterns for stripping
var (
	htmlTagPattern      = regexp.MustCompile(`<[^>]*>`)
	multiSpacePattern   = regexp.MustCompile(`[ \t]+`)
	multiNewlinePattern = regexp.MustCompile(`\n{3,}`)
)

// stripHTML removes HTML tags and decodes HTML entities.
func stripHTML(text string) string {
	// Remove HTML tags
	text = htmlTagPattern.ReplaceAllString(text, "")

	// Decode HTML entities
	text = html.UnescapeString(text)

	// Normalize whitespace
	text = multiSpacePattern.ReplaceAllString(text, " ")
	text = multiNewlinePattern.ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
