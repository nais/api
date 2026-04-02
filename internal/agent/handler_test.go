package agent

import (
	"encoding/json"
	"testing"
)

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
