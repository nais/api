package agent

import (
	"testing"

	"github.com/nais/api/internal/agent/chat"
)

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

func TestClientVisibleBlocks(t *testing.T) {
	blocks := []ContentBlock{
		{Type: ContentBlockTypeThinking, Thinking: "reasoning"},
		{Type: ContentBlockTypeText, Text: "here is the answer"},
		{Type: ContentBlockTypeToolUse, ToolCallID: "call_1", ToolName: "execute_graphql", ToolSuccess: true, ToolResult: `{"data":{}}`},
		{Type: ContentBlockTypeChart, Chart: &ChartData{Title: "CPU Usage"}},
	}

	got := clientVisibleBlocks(blocks)

	if len(got) != 3 {
		t.Fatalf("expected 3 visible blocks, got %d", len(got))
	}
	for _, b := range got {
		if b.Type == ContentBlockTypeToolUse {
			t.Errorf("clientVisibleBlocks() should not include tool_use blocks")
		}
	}
}
