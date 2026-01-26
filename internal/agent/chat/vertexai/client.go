// Package vertexai implements the chat.Client interface using Google Vertex AI Gemini.
package vertexai

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/sirupsen/logrus"
	"google.golang.org/genai"
)

const (
	defaultTemperature = 0.3
	defaultMaxTokens   = 4096
	defaultTopP        = 0.95
	defaultTopK        = 40
)

// Config holds configuration for the Vertex AI client.
type Config struct {
	ProjectID string
	Location  string
	ModelName string
}

// Client implements chat.StreamingClient using Vertex AI Gemini.
type Client struct {
	client    *genai.Client
	modelName string
	log       logrus.FieldLogger
}

// NewClient creates a new Vertex AI chat client.
func NewClient(ctx context.Context, cfg Config, log logrus.FieldLogger) (*Client, error) {
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

	return &Client{
		client:    client,
		modelName: cfg.ModelName,
		log:       log,
	}, nil
}

// Chat sends a message to the LLM and returns a response.
func (c *Client) Chat(ctx context.Context, req *chat.Request) (*chat.Response, error) {
	config := c.buildGenerateContentConfig(req)
	contents := buildContents(req)

	resp, err := c.client.Models.GenerateContent(ctx, c.modelName, contents, config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate content: %w", err)
	}

	return convertResponse(resp), nil
}

// ChatStream sends a message and returns a channel of response chunks.
func (c *Client) ChatStream(ctx context.Context, req *chat.Request) (<-chan chat.StreamChunk, error) {
	chunkCh := make(chan chat.StreamChunk, 100)

	go func() {
		defer close(chunkCh)

		config := c.buildGenerateContentConfig(req)
		contents := buildContents(req)

		for resp, err := range c.client.Models.GenerateContentStream(ctx, c.modelName, contents, config) {
			if err != nil {
				chunkCh <- chat.StreamChunk{Error: err}
				return
			}

			chunk := convertStreamResponse(resp)
			chunkCh <- chunk
		}

		chunkCh <- chat.StreamChunk{Done: true}
	}()

	return chunkCh, nil
}

// Close cleans up resources.
func (c *Client) Close() error {
	// The new SDK client doesn't have a Close method
	return nil
}

func (c *Client) buildGenerateContentConfig(req *chat.Request) *genai.GenerateContentConfig {
	temp := float32(defaultTemperature)
	topP := float32(defaultTopP)
	topK := float32(defaultTopK)

	config := &genai.GenerateContentConfig{
		Temperature:     &temp,
		MaxOutputTokens: int32(defaultMaxTokens),
		TopP:            &topP,
		TopK:            &topK,
		Tools:           convertTools(req.Tools),
	}

	if req.SystemPrompt != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: req.SystemPrompt}},
		}
	}

	return config
}

func buildContents(req *chat.Request) []*genai.Content {
	contents := make([]*genai.Content, 0, len(req.Messages))

	for _, msg := range req.Messages {
		content := &genai.Content{
			Role: convertRole(msg.Role),
		}

		switch msg.Role {
		case chat.RoleTool:
			// Tool response
			content.Role = "user"
			content.Parts = []*genai.Part{
				{
					FunctionResponse: &genai.FunctionResponse{
						Name:     msg.ToolCallID,
						Response: map[string]any{"result": msg.Content},
					},
				},
			}
		case chat.RoleAssistant:
			if len(msg.ToolCalls) > 0 {
				// Assistant message with tool calls
				parts := make([]*genai.Part, 0, len(msg.ToolCalls)+1)
				if msg.Content != "" {
					parts = append(parts, &genai.Part{Text: msg.Content})
				}
				for _, tc := range msg.ToolCalls {
					parts = append(parts, &genai.Part{
						FunctionCall: &genai.FunctionCall{
							Name: tc.Name,
							Args: tc.Arguments,
						},
					})
				}
				content.Parts = parts
			} else {
				content.Parts = []*genai.Part{{Text: msg.Content}}
			}
		default:
			content.Parts = []*genai.Part{{Text: msg.Content}}
		}

		contents = append(contents, content)
	}

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

func convertRole(role chat.Role) string {
	switch role {
	case chat.RoleUser:
		return "user"
	case chat.RoleAssistant:
		return "model"
	case chat.RoleTool:
		return "user"
	default:
		return "user"
	}
}

func convertTools(tools []chat.ToolDefinition) []*genai.Tool {
	if len(tools) == 0 {
		return nil
	}

	funcDecls := make([]*genai.FunctionDeclaration, 0, len(tools))
	for _, tool := range tools {
		funcDecl := &genai.FunctionDeclaration{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  convertParameters(tool.Parameters),
		}
		funcDecls = append(funcDecls, funcDecl)
	}

	return []*genai.Tool{
		{FunctionDeclarations: funcDecls},
	}
}

func convertParameters(params []chat.ParameterDefinition) *genai.Schema {
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

func convertResponse(resp *genai.GenerateContentResponse) *chat.Response {
	result := &chat.Response{
		Usage: &chat.UsageStats{},
	}

	if resp.UsageMetadata != nil {
		result.Usage.InputTokens = int(resp.UsageMetadata.PromptTokenCount)
		result.Usage.OutputTokens = int(resp.UsageMetadata.CandidatesTokenCount)
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
			result.Content += part.Text
		}
		if part.FunctionCall != nil {
			result.ToolCalls = append(result.ToolCalls, chat.ToolCall{
				ID:        part.FunctionCall.Name, // Use name as ID for Gemini
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			})
		}
	}

	return result
}

func convertStreamResponse(resp *genai.GenerateContentResponse) chat.StreamChunk {
	chunk := chat.StreamChunk{}

	if len(resp.Candidates) == 0 {
		return chunk
	}

	candidate := resp.Candidates[0]
	if candidate.Content == nil {
		return chunk
	}

	for _, part := range candidate.Content.Parts {
		if part.Text != "" {
			chunk.Content += part.Text
		}
		if part.FunctionCall != nil {
			chunk.ToolCalls = append(chunk.ToolCalls, chat.ToolCall{
				ID:        part.FunctionCall.Name,
				Name:      part.FunctionCall.Name,
				Arguments: part.FunctionCall.Args,
			})
		}
	}

	return chunk
}
