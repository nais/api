package agent

import (
	"encoding/json"
	"testing"

	"github.com/nais/api/internal/agent/chat"
)

func TestFilterUsedSources(t *testing.T) {
	tests := []struct {
		name     string
		response string
		sources  []Source
		want     []Source
	}{
		{
			name:     "empty sources returns empty",
			response: "Some response text",
			sources:  []Source{},
			want:     []Source{},
		},
		{
			name:     "nil sources returns nil",
			response: "Some response text",
			sources:  nil,
			want:     nil,
		},
		{
			name:     "exact title match",
			response: "According to the Deployment Guide, you should configure your app correctly.",
			sources: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
			},
			want: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
			},
		},
		{
			name:     "case insensitive title match",
			response: "The DEPLOYMENT GUIDE explains how to deploy applications.",
			sources: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
			},
			want: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
			},
		},
		{
			name:     "URL match",
			response: "For more information, see https://docs.nais.io/deployment",
			sources: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
			},
			want: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
			},
		},
		{
			name:     "keyword match from title",
			response: "When configuring alerts, you need to specify thresholds.",
			sources: []Source{
				{Title: "Nais Alerts Configuration", URL: "https://docs.nais.io/alerts"},
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
			},
			want: []Source{
				{Title: "Nais Alerts Configuration", URL: "https://docs.nais.io/alerts"},
			},
		},
		{
			name:     "multiple sources used",
			response: "For deployment, see the Deployment Guide. For security configuration, check the Security Guide.",
			sources: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
				{Title: "Monitoring Guide", URL: "https://docs.nais.io/monitoring"},
			},
			want: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
			},
		},
		{
			name:     "no sources match",
			response: "I don't have specific information about that topic.",
			sources: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
			},
			want: []Source{},
		},
		{
			name:     "skips common words like guide and documentation",
			response: "Here is a guide on how to use the platform documentation.",
			sources: []Source{
				{Title: "Guide Documentation", URL: "https://docs.nais.io/guide"},
			},
			want: []Source{},
		},
		{
			name:     "matches on significant keywords",
			response: "To configure observability for your application, you need to set up metrics.",
			sources: []Source{
				{Title: "Observability Setup", URL: "https://docs.nais.io/observability"},
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
			},
			want: []Source{
				{Title: "Observability Setup", URL: "https://docs.nais.io/observability"},
			},
		},
		{
			name:     "all sources used",
			response: "The Deployment Guide and Security Guide both explain important concepts for production.",
			sources: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
			},
			want: []Source{
				{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
				{Title: "Security Guide", URL: "https://docs.nais.io/security"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterUsedSources(tt.response, tt.sources)

			if len(got) != len(tt.want) {
				t.Errorf("filterUsedSources() returned %d sources, want %d", len(got), len(tt.want))
				t.Errorf("got: %v", got)
				t.Errorf("want: %v", tt.want)
				return
			}

			for i, source := range got {
				if source.Title != tt.want[i].Title || source.URL != tt.want[i].URL {
					t.Errorf("filterUsedSources()[%d] = %v, want %v", i, source, tt.want[i])
				}
			}
		})
	}
}

func TestExtractToolCallsFromBlocks(t *testing.T) {
	tests := []struct {
		name   string
		blocks []ContentBlock
		want   []chat.ToolCall
	}{
		{
			name:   "empty blocks",
			blocks: []ContentBlock{},
			want:   nil,
		},
		{
			name: "no tool blocks",
			blocks: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Hello"},
				{Type: ContentBlockTypeThinking, Thinking: "Thinking..."},
			},
			want: nil,
		},
		{
			name: "single tool call",
			blocks: []ContentBlock{
				{Type: ContentBlockTypeToolUse, ToolCallID: "call_1", ToolName: "execute_graphql", ToolSuccess: true, ToolResult: "result data"},
			},
			want: []chat.ToolCall{
				{ID: "call_1", Name: "execute_graphql"},
			},
		},
		{
			name: "multiple tool calls in order",
			blocks: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Let me check..."},
				{Type: ContentBlockTypeToolUse, ToolCallID: "call_1", ToolName: "schema_get_type", ToolSuccess: true, ToolResult: "type info"},
				{Type: ContentBlockTypeToolUse, ToolCallID: "call_2", ToolName: "execute_graphql", ToolSuccess: true, ToolResult: "query result"},
				{Type: ContentBlockTypeText, Text: "Here's what I found..."},
			},
			want: []chat.ToolCall{
				{ID: "call_1", Name: "schema_get_type"},
				{ID: "call_2", Name: "execute_graphql"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolCallsFromBlocks(tt.blocks)
			if len(got) != len(tt.want) {
				t.Errorf("extractToolCallsFromBlocks() returned %d tool calls, want %d", len(got), len(tt.want))
				return
			}
			for i, tc := range got {
				if tc.ID != tt.want[i].ID || tc.Name != tt.want[i].Name {
					t.Errorf("extractToolCallsFromBlocks()[%d] = {ID: %q, Name: %q}, want {ID: %q, Name: %q}",
						i, tc.ID, tc.Name, tt.want[i].ID, tt.want[i].Name)
				}
			}
		})
	}
}

func TestContentBlockJSONRoundTrip(t *testing.T) {
	// Test that ToolResult is properly serialized/deserialized in blocks
	blocks := []ContentBlock{
		{Type: ContentBlockTypeText, Text: "Checking data..."},
		{
			Type:        ContentBlockTypeToolUse,
			ToolCallID:  "call_abc123",
			ToolName:    "execute_graphql",
			ToolSuccess: true,
			ToolResult:  `{"data":{"team":{"slug":"my-team"}}}`,
		},
		{Type: ContentBlockTypeText, Text: "Found your team!"},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(blocks)
	if err != nil {
		t.Fatalf("failed to marshal blocks: %v", err)
	}

	// Unmarshal back
	var restored []ContentBlock
	if err := json.Unmarshal(jsonData, &restored); err != nil {
		t.Fatalf("failed to unmarshal blocks: %v", err)
	}

	// Verify
	if len(restored) != len(blocks) {
		t.Fatalf("got %d blocks, want %d", len(restored), len(blocks))
	}

	toolBlock := restored[1]
	if toolBlock.Type != ContentBlockTypeToolUse {
		t.Errorf("block type = %v, want %v", toolBlock.Type, ContentBlockTypeToolUse)
	}
	if toolBlock.ToolCallID != "call_abc123" {
		t.Errorf("ToolCallID = %q, want %q", toolBlock.ToolCallID, "call_abc123")
	}
	if toolBlock.ToolName != "execute_graphql" {
		t.Errorf("ToolName = %q, want %q", toolBlock.ToolName, "execute_graphql")
	}
	if !toolBlock.ToolSuccess {
		t.Error("ToolSuccess = false, want true")
	}
	if toolBlock.ToolResult != `{"data":{"team":{"slug":"my-team"}}}` {
		t.Errorf("ToolResult = %q, want JSON data", toolBlock.ToolResult)
	}
}

func TestExtractTextContentFromBlocks(t *testing.T) {
	tests := []struct {
		name   string
		blocks []ContentBlock
		want   string
	}{
		{
			name:   "empty blocks",
			blocks: []ContentBlock{},
			want:   "",
		},
		{
			name: "single text block",
			blocks: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "Hello, world!"},
			},
			want: "Hello, world!",
		},
		{
			name: "multiple text blocks",
			blocks: []ContentBlock{
				{Type: ContentBlockTypeText, Text: "First part."},
				{Type: ContentBlockTypeToolUse, ToolName: "some_tool"},
				{Type: ContentBlockTypeText, Text: "Second part."},
			},
			want: "First part. Second part.",
		},
		{
			name: "ignores non-text blocks",
			blocks: []ContentBlock{
				{Type: ContentBlockTypeThinking, Thinking: "Thinking..."},
				{Type: ContentBlockTypeToolUse, ToolName: "tool", ToolResult: "result"},
				{Type: ContentBlockTypeChart, Chart: &ChartData{Title: "Chart"}},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextContentFromBlocks(tt.blocks)
			if got != tt.want {
				t.Errorf("extractTextContentFromBlocks() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterUsedSources_PreservesOrder(t *testing.T) {
	response := "First check the Security Guide, then the Deployment Guide, finally Monitoring Guide."
	sources := []Source{
		{Title: "Deployment Guide", URL: "https://docs.nais.io/deployment"},
		{Title: "Security Guide", URL: "https://docs.nais.io/security"},
		{Title: "Monitoring Guide", URL: "https://docs.nais.io/monitoring"},
	}

	got := filterUsedSources(response, sources)

	// Should preserve the original order from sources slice, not the order mentioned in response
	if len(got) != 3 {
		t.Fatalf("expected 3 sources, got %d", len(got))
	}
	if got[0].Title != "Deployment Guide" {
		t.Errorf("expected first source to be Deployment Guide, got %s", got[0].Title)
	}
	if got[1].Title != "Security Guide" {
		t.Errorf("expected second source to be Security Guide, got %s", got[1].Title)
	}
	if got[2].Title != "Monitoring Guide" {
		t.Errorf("expected third source to be Monitoring Guide, got %s", got[2].Title)
	}
}
