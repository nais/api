package gen_rag_index

import (
	"testing"
)

func TestStripHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple tags",
			input:    "<p>Hello <b>world</b></p>",
			expected: "Hello world",
		},
		{
			name:     "nested tags",
			input:    "<div><ul><li>Item 1</li><li>Item 2</li></ul></div>",
			expected: "Item 1Item 2",
		},
		{
			name:     "html entities",
			input:    "Hello &amp; goodbye &lt;test&gt;",
			expected: "Hello & goodbye <test>",
		},
		{
			name:     "mixed content",
			input:    "<p>This is <code>code</code> and &quot;quotes&quot;</p>",
			expected: "This is code and \"quotes\"",
		},
		{
			name:     "multiple spaces",
			input:    "Hello    world   test",
			expected: "Hello world test",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripHTML(tt.input)
			if result != tt.expected {
				t.Errorf("stripHTML(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetPagePath(t *testing.T) {
	tests := []struct {
		name     string
		location string
		expected string
	}{
		{
			name:     "no fragment",
			location: "auth/entra-id/",
			expected: "auth/entra-id/",
		},
		{
			name:     "with fragment",
			location: "auth/entra-id/#spec",
			expected: "auth/entra-id/",
		},
		{
			name:     "empty string",
			location: "",
			expected: "",
		},
		{
			name:     "only fragment",
			location: "#welcome",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getPagePath(tt.location)
			if result != tt.expected {
				t.Errorf("getPagePath(%q) = %q, want %q", tt.location, result, tt.expected)
			}
		})
	}
}

func TestIsTagIndexPage(t *testing.T) {
	tests := []struct {
		name     string
		pagePath string
		location string
		expected bool
	}{
		{
			name:     "tags main page",
			pagePath: "tags/",
			location: "tags/",
			expected: true,
		},
		{
			name:     "tag subpage",
			pagePath: "tags/",
			location: "tags/#tag:how-to",
			expected: true,
		},
		{
			name:     "regular page",
			pagePath: "auth/",
			location: "auth/",
			expected: false,
		},
		{
			name:     "regular page with fragment",
			pagePath: "auth/",
			location: "auth/#logging-in-users",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTagIndexPage(tt.pagePath, tt.location)
			if result != tt.expected {
				t.Errorf("isTagIndexPage(%q, %q) = %v, want %v", tt.pagePath, tt.location, result, tt.expected)
			}
		})
	}
}

func TestBuildURL(t *testing.T) {
	tests := []struct {
		name     string
		baseURL  string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			baseURL:  "https://docs.example.cloud.nais.io",
			path:     "auth/",
			expected: "https://docs.example.cloud.nais.io/auth/",
		},
		{
			name:     "empty path",
			baseURL:  "https://docs.example.cloud.nais.io",
			path:     "",
			expected: "https://docs.example.cloud.nais.io/",
		},
		{
			name:     "nested path",
			baseURL:  "https://docs.example.cloud.nais.io",
			path:     "auth/entra-id/reference/",
			expected: "https://docs.example.cloud.nais.io/auth/entra-id/reference/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildURL(tt.baseURL, tt.path)
			if result != tt.expected {
				t.Errorf("buildURL(%q, %q) = %q, want %q", tt.baseURL, tt.path, result, tt.expected)
			}
		})
	}
}

func TestMergeTags(t *testing.T) {
	tests := []struct {
		name     string
		existing []string
		new      []string
		expected []string
	}{
		{
			name:     "no overlap",
			existing: []string{"a", "b"},
			new:      []string{"c", "d"},
			expected: []string{"a", "b", "c", "d"},
		},
		{
			name:     "with overlap",
			existing: []string{"a", "b"},
			new:      []string{"b", "c"},
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "empty existing",
			existing: []string{},
			new:      []string{"a", "b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "empty new",
			existing: []string{"a", "b"},
			new:      []string{},
			expected: []string{"a", "b"},
		},
		{
			name:     "both empty",
			existing: []string{},
			new:      []string{},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeTags(tt.existing, tt.new)
			if len(result) != len(tt.expected) {
				t.Errorf("mergeTags(%v, %v) = %v, want %v", tt.existing, tt.new, result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("mergeTags(%v, %v) = %v, want %v", tt.existing, tt.new, result, tt.expected)
					return
				}
			}
		})
	}
}

func TestBuildChunkHeader(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		tags     []string
		expected string
	}{
		{
			name:     "with tags",
			title:    "My Page",
			tags:     []string{"auth", "how-to"},
			expected: "Title: My Page\nTags: auth, how-to\n\n",
		},
		{
			name:     "without tags",
			title:    "My Page",
			tags:     nil,
			expected: "Title: My Page\n\n",
		},
		{
			name:     "empty tags",
			title:    "My Page",
			tags:     []string{},
			expected: "Title: My Page\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildChunkHeader(tt.title, tt.tags)
			if result != tt.expected {
				t.Errorf("buildChunkHeader(%q, %v) = %q, want %q", tt.title, tt.tags, result, tt.expected)
			}
		})
	}
}

func TestProcessSearchIndex(t *testing.T) {
	index := &SearchIndex{
		Docs: []SearchDoc{
			{
				Location: "auth/",
				Title:    "Authentication",
				Text:     "<p>Auth overview</p>",
				Tags:     []string{"auth", "explanation"},
			},
			{
				Location: "auth/#logging-in",
				Title:    "Logging in",
				Text:     "<p>How to log in</p>",
				Tags:     []string{"auth", "explanation"},
			},
			{
				Location: "tags/",
				Title:    "Tags",
				Text:     "",
			},
			{
				Location: "tags/#tag:how-to",
				Title:    "how-to",
				Text:     "<ul><li>Item</li></ul>",
			},
		},
	}

	pages := ProcessSearchIndex(index, "https://docs.test.cloud.nais.io")

	// Should have 1 page (auth/), tags should be skipped
	if len(pages) != 1 {
		t.Errorf("ProcessSearchIndex() returned %d pages, want 1", len(pages))
		return
	}

	page := pages[0]
	if page.Path != "auth/" {
		t.Errorf("page.Path = %q, want %q", page.Path, "auth/")
	}
	if page.Title != "Authentication" {
		t.Errorf("page.Title = %q, want %q", page.Title, "Authentication")
	}
	if len(page.Tags) != 2 {
		t.Errorf("page.Tags = %v, want 2 tags", page.Tags)
	}
	if len(page.Sections) != 2 {
		t.Errorf("page.Sections = %d, want 2", len(page.Sections))
	}
	if page.URL != "https://docs.test.cloud.nais.io/auth/" {
		t.Errorf("page.URL = %q, want %q", page.URL, "https://docs.test.cloud.nais.io/auth/")
	}
}

func TestChunkPages(t *testing.T) {
	pages := []Page{
		{
			Path:  "short/",
			Title: "Short Page",
			URL:   "https://example.com/short/",
			Tags:  []string{"test"},
			Sections: []Section{
				{Title: "Short Page", Text: "This is short content."},
			},
		},
	}

	chunks := ChunkPages(pages, 1500)

	if len(chunks) != 1 {
		t.Errorf("ChunkPages() returned %d chunks, want 1", len(chunks))
		return
	}

	chunk := chunks[0]
	if chunk.Title != "Short Page" {
		t.Errorf("chunk.Title = %q, want %q", chunk.Title, "Short Page")
	}
	if chunk.URL != "https://example.com/short/" {
		t.Errorf("chunk.URL = %q, want %q", chunk.URL, "https://example.com/short/")
	}

	// Should contain header with title and tags
	if !contains(chunk.Content, "Title: Short Page") {
		t.Errorf("chunk.Content should contain title header")
	}
	if !contains(chunk.Content, "Tags: test") {
		t.Errorf("chunk.Content should contain tags header")
	}
	if !contains(chunk.Content, "This is short content.") {
		t.Errorf("chunk.Content should contain the actual content")
	}
}

func TestChunkPagesLongContent(t *testing.T) {
	// Create content that's longer than chunk size
	longContent := ""
	for i := 0; i < 100; i++ {
		longContent += "This is a sentence that adds length to the content. "
	}

	pages := []Page{
		{
			Path:  "long/",
			Title: "Long Page",
			URL:   "https://example.com/long/",
			Sections: []Section{
				{Title: "Long Page", Text: longContent},
			},
		},
	}

	chunks := ChunkPages(pages, 500)

	if len(chunks) < 2 {
		t.Errorf("ChunkPages() should split long content into multiple chunks, got %d", len(chunks))
	}

	// All chunks should have the URL
	for _, chunk := range chunks {
		if chunk.URL != "https://example.com/long/" {
			t.Errorf("all chunks should have the same URL")
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
