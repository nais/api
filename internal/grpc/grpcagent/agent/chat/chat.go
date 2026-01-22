// Package chat defines interfaces and types for LLM chat interactions.
package chat

import (
	"context"

	"github.com/nais/api/internal/grpc/grpcagent/agent/rag"
)

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Client defines the interface for LLM chat interactions.
type Client interface {
	// Chat sends a message to the LLM and returns a response.
	// The response may contain tool calls that need to be executed.
	Chat(ctx context.Context, req *Request) (*Response, error)

	// Close cleans up any resources held by the client.
	Close() error
}

// StreamingClient extends Client with streaming support.
type StreamingClient interface {
	Client

	// ChatStream sends a message and returns a channel of response chunks.
	// The channel is closed when the response is complete or an error occurs.
	ChatStream(ctx context.Context, req *Request) (<-chan StreamChunk, error)
}

// Request represents a chat request to the LLM.
type Request struct {
	// SystemPrompt is the system-level instruction for the LLM.
	SystemPrompt string

	// Messages is the conversation history.
	Messages []Message

	// Tools is the list of tools available to the LLM.
	Tools []ToolDefinition

	// Documents contains RAG results to include in context.
	Documents []rag.Document
}

// Response represents a chat response from the LLM.
type Response struct {
	// Content is the text response from the LLM.
	Content string

	// ToolCalls contains any tool calls the LLM wants to make.
	ToolCalls []ToolCall

	// Usage contains token usage statistics.
	Usage *UsageStats
}

// StreamChunk represents a chunk of a streaming response.
type StreamChunk struct {
	// Content is the partial text content.
	Content string

	// ToolCalls contains tool calls (sent as complete objects).
	ToolCalls []ToolCall

	// Done is true if this is the final chunk.
	Done bool

	// Error is non-nil if an error occurred.
	Error error
}

// Message represents a single message in the conversation.
type Message struct {
	// Role is the sender role (user, assistant, or tool).
	Role Role

	// Content is the message text.
	Content string

	// ToolCallID is set for tool response messages to indicate which tool call this responds to.
	ToolCallID string

	// ToolCalls is set for assistant messages that request tool calls.
	ToolCalls []ToolCall
}

// ToolCall represents a tool invocation requested by the LLM.
type ToolCall struct {
	// ID is a unique identifier for this tool call.
	ID string

	// Name is the name of the tool to invoke.
	Name string

	// Arguments contains the arguments for the tool.
	Arguments map[string]any
}

// ToolDefinition describes a tool available to the LLM.
type ToolDefinition struct {
	// Name is the unique name of the tool.
	Name string

	// Description explains what the tool does.
	Description string

	// Parameters describes the tool's parameters.
	Parameters []ParameterDefinition
}

// ParameterDefinition describes a single parameter for a tool.
type ParameterDefinition struct {
	// Name is the parameter name.
	Name string

	// Type is the parameter type (e.g., "string", "object", "array").
	Type string

	// Description explains what the parameter is for.
	Description string

	// Required indicates if the parameter must be provided.
	Required bool
}

// UsageStats contains token usage information.
type UsageStats struct {
	// InputTokens is the number of tokens in the input.
	InputTokens int

	// OutputTokens is the number of tokens in the output.
	OutputTokens int
}
