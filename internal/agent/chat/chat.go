// Package chat provides the LLM chat client for the agent using Vertex AI Gemini.
package chat

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/agent/rag"
	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

// EmbeddingConfig holds configuration for the Vertex AI embedding client.
type EmbeddingConfig struct {
	// ProjectID is the GCP project ID.
	ProjectID string

	// Location is the GCP region (must be in EU, e.g., "europe-west1").
	Location string

	// ModelName is the embedding model to use (e.g., "gemini-embedding-001").
	ModelName string
}

// EmbeddingClient generates embeddings using Vertex AI.
type EmbeddingClient struct {
	client *genai.Client
	model  string
	log    logrus.FieldLogger
}

// NewEmbeddingClient creates a new Vertex AI embedding client.
func NewEmbeddingClient(ctx context.Context, cfg EmbeddingConfig, log logrus.FieldLogger) (*EmbeddingClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  cfg.ProjectID,
		Location: cfg.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	log.WithFields(logrus.Fields{
		"project":  cfg.ProjectID,
		"location": cfg.Location,
		"model":    cfg.ModelName,
	}).Info("initialized Vertex AI embedding client")

	return &EmbeddingClient{
		client: client,
		model:  cfg.ModelName,
		log:    log,
	}, nil
}

// Embed returns the embedding vector for the given text.
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := c.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	return embeddings[0], nil
}

// EmbedBatch returns embedding vectors for multiple texts in a single API call.
// This is much more efficient than calling Embed multiple times.
// The Vertex AI API supports up to 250 texts per batch request.
func (c *EmbeddingClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = &genai.Content{Parts: []*genai.Part{{Text: text}}}
	}

	result, err := c.client.Models.EmbedContent(ctx, c.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to embed texts: %w", err)
	}

	if result == nil || len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}

	embeddings := make([][]float32, len(texts))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}

	return embeddings, nil
}

// Close cleans up resources.
func (c *EmbeddingClient) Close() error {
	// The genai client doesn't have a Close method
	return nil
}

const (
	defaultTemperature = 0.3
	defaultMaxTokens   = 4096
	defaultTopP        = 0.95
	defaultTopK        = 40
)

// Role represents the role of a message sender.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Client defines the interface for LLM chat interactions.
// This interface exists primarily for testing purposes.
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

	// Thinking is the model's reasoning/thought process (only populated when thinking mode is enabled).
	Thinking string

	// ToolCalls contains any tool calls the LLM wants to make.
	ToolCalls []ToolCall

	// Usage contains token usage statistics.
	Usage *UsageStats
}

// StreamChunk represents a chunk of a streaming response.
type StreamChunk struct {
	// Content is the partial text content.
	Content string

	// Thinking is the model's reasoning/thought process (only populated when thinking mode is enabled).
	Thinking string

	// ToolCalls contains tool calls (sent as complete objects).
	ToolCalls []ToolCall

	// Usage contains token usage statistics.
	Usage *UsageStats

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

	// ThoughtSignature is an encrypted representation of the model's internal thought process.
	// Required by Gemini 3 models for function calling - must be preserved and sent back
	// when providing function responses.
	ThoughtSignature string `json:"thought_signature,omitempty"`
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
	InputTokens int `json:"input_tokens"`

	// OutputTokens is the number of tokens in the output.
	OutputTokens int `json:"output_tokens"`

	// TotalTokens is the total number of tokens used (input + output).
	TotalTokens int `json:"total_tokens"`

	// MaxTokens is the maximum number of tokens allowed for the model/context window.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// Config holds configuration for the Vertex AI chat client.
type Config struct {
	ProjectID       string
	Location        string
	ModelName       string
	IncludeThoughts bool // Include thought content in responses
}

// VertexAIClient implements StreamingClient using Vertex AI Gemini.
type VertexAIClient struct {
	client          *genai.Client
	modelName       string
	log             logrus.FieldLogger
	includeThoughts bool
}

// NewClient creates a new Vertex AI chat client.
func NewClient(ctx context.Context, cfg Config, log logrus.FieldLogger) (*VertexAIClient, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Project:  cfg.ProjectID,
		Location: cfg.Location,
		Backend:  genai.BackendVertexAI,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Vertex AI client: %w", err)
	}

	log.WithFields(logrus.Fields{
		"project":  cfg.ProjectID,
		"location": cfg.Location,
		"model":    cfg.ModelName,
	}).Info("initialized Vertex AI chat client")

	return &VertexAIClient{
		client:          client,
		modelName:       cfg.ModelName,
		log:             log,
		includeThoughts: cfg.IncludeThoughts,
	}, nil
}

// Chat sends a message to the LLM and returns a response.
func (c *VertexAIClient) Chat(ctx context.Context, req *Request) (*Response, error) {
	config := c.buildGenerateContentConfig(req)
	contents := c.buildContents(req)

	resp, err := c.client.Models.GenerateContent(ctx, c.modelName, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	return c.convertResponse(resp), nil
}

// ChatStream sends a message and returns a channel of response chunks.
func (c *VertexAIClient) ChatStream(ctx context.Context, req *Request) (<-chan StreamChunk, error) {
	chunkCh := make(chan StreamChunk, 100)

	go func() {
		defer close(chunkCh)

		config := c.buildGenerateContentConfig(req)
		contents := c.buildContents(req)

		for resp, err := range c.client.Models.GenerateContentStream(ctx, c.modelName, contents, config) {
			if err != nil {
				chunkCh <- StreamChunk{Error: err}
				return
			}

			chunk := c.convertStreamResponse(resp)
			chunkCh <- chunk
		}

		chunkCh <- StreamChunk{Done: true}
	}()

	return chunkCh, nil
}

// Close cleans up resources.
func (c *VertexAIClient) Close() error {
	// The genai client doesn't have a Close method
	return nil
}

func (c *VertexAIClient) buildGenerateContentConfig(req *Request) *genai.GenerateContentConfig {
	temp := float32(defaultTemperature)
	topP := float32(defaultTopP)
	topK := float32(defaultTopK)

	config := &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: int32(defaultMaxTokens),
		TopP:            &topP,
		TopK:            &topK,
		Tools:           c.convertTools(req.Tools),
	}

	if req.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: req.SystemPrompt}},
		}
	}

	// Enable thinking mode for Gemini 3+ models
	// This is required for proper thought signature handling
	config.ThinkingConfig = &genai.ThinkingConfig{
		IncludeThoughts: c.includeThoughts,
	}

	return config
}

func (c *VertexAIClient) buildContents(req *Request) []*genai.Content {
	contents := make([]*genai.Content, 0, len(req.Messages))

	// Collect consecutive tool responses to batch them into a single content block.
	// Vertex AI/Gemini requires all function responses for a turn to be in one content.
	var pendingToolResponses []*genai.Part

	flushToolResponses := func() {
		if len(pendingToolResponses) > 0 {
			contents = append(contents, &genai.Content{
				Role:  "user",
				Parts: pendingToolResponses,
			})
			pendingToolResponses = nil
		}
	}

	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleTool:
			// Collect tool response - will be batched with other consecutive tool responses
			pendingToolResponses = append(pendingToolResponses, &genai.Part{
				FunctionResponse: &genai.FunctionResponse{
					Name:     msg.ToolCallID,
					Response: map[string]any{"result": msg.Content},
				},
			})
		case RoleAssistant:
			// Flush any pending tool responses before adding assistant message
			flushToolResponses()

			content := &genai.Content{
				Role: "model",
			}
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool calls
				parts := make([]*genai.Part, 0, len(msg.ToolCalls)+1)
				if msg.Content != "" {
					parts = append(parts, &genai.Part{Text: msg.Content})
				}
				for _, tc := range msg.ToolCalls {
					part := &genai.Part{
						FunctionCall: &genai.FunctionCall{
							Name: tc.Name,
							Args: tc.Arguments,
						},
					}
					// Preserve thought signature for Gemini 3 models
					if tc.ThoughtSignature != "" {
						part.ThoughtSignature = []byte(tc.ThoughtSignature)
					}
					parts = append(parts, part)
				}
				content.Parts = parts
			} else {
				content.Parts = []*genai.Part{{Text: msg.Content}}
			}
			contents = append(contents, content)
		default:
			// Flush any pending tool responses before adding user message
			flushToolResponses()

			content := &genai.Content{
				Role:  "user",
				Parts: []*genai.Part{{Text: msg.Content}},
			}
			contents = append(contents, content)
		}
	}

	// Flush any remaining tool responses at the end
	flushToolResponses()

	// Add document context if available
	if len(req.Documents) > 0 {
		docContext := formatDocuments(req.Documents)
		if len(contents) > 0 && contents[len(contents)-1].Role == "user" {
			// Prepend to last user message
			lastContent := contents[len(contents)-1]
			newParts := make([]*genai.Part, 0, len(lastContent.Parts)+1)
			newParts = append(newParts, &genai.Part{Text: docContext + "\n\n"})
			newParts = append(newParts, lastContent.Parts...)
			lastContent.Parts = newParts
		}
	}

	return contents
}

func formatDocuments(docs []rag.Document) string {
	if len(docs) == 0 {
		return ""
	}

	result := "Here is relevant documentation:\n\n"
	for _, doc := range docs {
		result += fmt.Sprintf("### %s\n%s\nSource: %s\n\n", doc.Title, doc.Content, doc.URL)
	}
	return result
}

func (c *VertexAIClient) convertTools(tools []ToolDefinition) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	funcDecls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  c.convertParameters(tool.Parameters),
		}
		funcDecls = append(funcDecls, funcDecl)
	}

	return []*genai.Tool{
		{FunctionDeclarations: funcDecls},
	}
}

func (c *VertexAIClient) convertParameters(params []ParameterDefinition) *genai.Schema {
	if len(params) == 0 {
		return nil
	}

	properties := make(map[string]*genai.Schema)
	required := make([]string, 0)

	for _, param := range params {
		properties[param.Name] = &genai.Schema{
			Type:        convertType(param.Type),
			Description: param.Description,
		}
		if param.Required {
			required = append(required, param.Name)
		}
	}

	return &genai.Schema{
		Type:       genai.TypeObject,
		Properties: properties,
		Required:   required,
	}
}

func convertType(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}

func (c *VertexAIClient) convertResponse(resp *genai.GenerateContentResponse) *Response {
	result := &Response{
		Usage: &UsageStats{},
	}

	if resp.UsageMetadata != nil {
		result.Usage.InputTokens = int(resp.UsageMetadata.PromptTokenCount)
		result.Usage.OutputTokens = int(resp.UsageMetadata.CandidatesTokenCount)
		result.Usage.TotalTokens = int(resp.UsageMetadata.PromptTokenCount + resp.UsageMetadata.CandidatesTokenCount)
		result.Usage.MaxTokens = c.getMaxContextWindow()
	}

	if len(resp.Candidates) == 0 {
		return result
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return result
	}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			// Check if this is a thought part (model's reasoning)
			if part.Thought {
				result.Thinking += part.Text
			} else {
				result.Content += part.Text
			}
		}
		if part.FunctionCall != nil {
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:               part.FunctionCall.Name, // Use name as ID for Gemini
				Name:             part.FunctionCall.Name,
				Arguments:        part.FunctionCall.Args,
				ThoughtSignature: string(part.ThoughtSignature), // Preserve thought signature for Gemini 3
			})
		}
	}

	return result
}

func (c *VertexAIClient) convertStreamResponse(resp *genai.GenerateContentResponse) StreamChunk {
	chunk := StreamChunk{}

	if resp.UsageMetadata != nil {
		chunk.Usage = &UsageStats{
			InputTokens:  int(resp.UsageMetadata.PromptTokenCount),
			OutputTokens: int(resp.UsageMetadata.CandidatesTokenCount),
			TotalTokens:  int(resp.UsageMetadata.PromptTokenCount + resp.UsageMetadata.CandidatesTokenCount),
			MaxTokens:    c.getMaxContextWindow(),
		}
	}

	if len(resp.Candidates) == 0 {
		return chunk
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return chunk
	}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			// Check if this is a thought part (model's reasoning)
			if part.Thought {
				chunk.Thinking += part.Text
			} else {
				chunk.Content += part.Text
			}
		}
		if part.FunctionCall != nil {
			chunk.ToolCalls = append(chunk.ToolCalls, ToolCall{
				ID:               part.FunctionCall.Name,
				Name:             part.FunctionCall.Name,
				Arguments:        part.FunctionCall.Args,
				ThoughtSignature: string(part.ThoughtSignature), // Preserve thought signature for Gemini 3
			})
		}
	}

	return chunk
}

func (c *VertexAIClient) getMaxContextWindow() int {
	// Default to 128k as a safe middle ground for modern models if unknown
	return 131072
}
