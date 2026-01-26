package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/sirupsen/logrus"
)

const (
	maxToolIterations = 5
	requestTimeout    = 60 * time.Second
	retryAttempts     = 3
	retryBaseDelay    = 100 * time.Millisecond
)

// OrchestratorEventType represents the type of streaming event from the orchestrator.
type OrchestratorEventType int

const (
	OrchestratorEventToolStart OrchestratorEventType = iota
	OrchestratorEventToolEnd
	OrchestratorEventContent
	OrchestratorEventError
)

// OrchestratorStreamEvent represents an event in the streaming response from the orchestrator.
type OrchestratorStreamEvent struct {
	Type        OrchestratorEventType
	Content     string
	ToolName    string
	Description string
	Success     bool
	Error       error
}

// OrchestratorResult contains the result of an orchestrated conversation.
type OrchestratorResult struct {
	Content   string
	ToolsUsed []ToolUsed
	Usage     *chat.UsageStats
}

// Orchestrator manages the conversation loop between user, LLM, and tools.
type Orchestrator struct {
	chatClient chat.StreamingClient
	naisAPIURL string
	authHeader string
	httpClient *http.Client
	log        logrus.FieldLogger
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(
	chatClient chat.StreamingClient,
	naisAPIURL string,
	authHeader string,
	log logrus.FieldLogger,
) *Orchestrator {
	return &Orchestrator{
		chatClient: chatClient,
		naisAPIURL: naisAPIURL,
		authHeader: authHeader,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		log: log,
	}
}

// Run executes a non-streaming conversation.
func (o *Orchestrator) Run(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	conversationID uuid.UUID,
) (*OrchestratorResult, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	o.log.WithFields(logrus.Fields{
		"conversation_id": conversationID,
		"message_length":  len(userMessage),
		"doc_count":       len(docs),
	}).Debug("starting orchestrator run")

	// Build system prompt
	systemPrompt := buildSystemPrompt(chatCtx, docs)
	o.log.WithField("prompt_length", len(systemPrompt)).Debug("built system prompt")

	// Initialize messages with user message
	messages := []chat.Message{
		{
			Role:    chat.RoleUser,
			Content: userMessage,
		},
	}

	var toolsUsed []ToolUsed
	var totalUsage chat.UsageStats

	// Tool execution loop
	for iteration := 0; iteration < maxToolIterations; iteration++ {
		o.log.WithField("iteration", iteration).Debug("starting tool iteration")
		// Build request
		req := &chat.Request{
			SystemPrompt: systemPrompt,
			Messages:     messages,
			Tools:        getToolDefinitions(),
			Documents:    docs,
		}

		// Call LLM with retries
		o.log.Debug("calling LLM")
		resp, err := o.callLLMWithRetry(ctx, req)
		if err != nil {
			o.log.WithError(err).Error("LLM call failed")
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		// Accumulate usage
		if resp.Usage != nil {
			totalUsage.InputTokens += resp.Usage.InputTokens
			totalUsage.OutputTokens += resp.Usage.OutputTokens
			o.log.WithFields(logrus.Fields{
				"input_tokens":  resp.Usage.InputTokens,
				"output_tokens": resp.Usage.OutputTokens,
			}).Debug("LLM usage stats")
		}

		o.log.WithFields(logrus.Fields{
			"content_length": len(resp.Content),
			"tool_calls":     len(resp.ToolCalls),
		}).Debug("LLM response received")

		// If no tool calls, we're done
		if len(resp.ToolCalls) == 0 {
			o.log.Debug("no tool calls, returning response")
			return &OrchestratorResult{
				Content:   resp.Content,
				ToolsUsed: toolsUsed,
				Usage:     &totalUsage,
			}, nil
		}

		// Add assistant message with tool calls
		messages = append(messages, chat.Message{
			Role:      chat.RoleAssistant,
			Content:   resp.Content,
			ToolCalls: resp.ToolCalls,
		})

		// Execute tool calls
		for _, toolCall := range resp.ToolCalls {
			o.log.WithFields(logrus.Fields{
				"tool":      toolCall.Name,
				"tool_id":   toolCall.ID,
				"iteration": iteration,
			}).Debug("executing tool call")

			toolResult, err := o.executeTool(ctx, toolCall)

			toolsUsed = append(toolsUsed, ToolUsed{
				Name:        toolCall.Name,
				Description: fmt.Sprintf("Executed %s", toolCall.Name),
			})

			var resultContent string
			if err != nil {
				resultContent = fmt.Sprintf("Error executing tool: %s", err.Error())
				o.log.WithError(err).WithField("tool", toolCall.Name).Warn("tool execution failed")
			} else {
				o.log.WithFields(logrus.Fields{
					"tool":          toolCall.Name,
					"result_length": len(toolResult),
				}).Debug("tool execution succeeded")
				resultContent = toolResult
			}

			// Add tool result message
			messages = append(messages, chat.Message{
				Role:       chat.RoleTool,
				Content:    resultContent,
				ToolCallID: toolCall.ID,
			})
		}
	}

	o.log.WithField("max_iterations", maxToolIterations).Error("max tool iterations exceeded")
	return nil, fmt.Errorf("max tool iterations (%d) exceeded", maxToolIterations)
}

// RunStream executes a streaming conversation.
func (o *Orchestrator) RunStream(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	conversationID uuid.UUID,
) (<-chan OrchestratorStreamEvent, error) {
	eventCh := make(chan OrchestratorStreamEvent, 100)

	o.log.WithFields(logrus.Fields{
		"conversation_id": conversationID,
		"message_length":  len(userMessage),
		"doc_count":       len(docs),
	}).Debug("starting streaming orchestrator")

	go func() {
		defer close(eventCh)

		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		// Build system prompt
		systemPrompt := buildSystemPrompt(chatCtx, docs)
		o.log.WithField("prompt_length", len(systemPrompt)).Debug("built system prompt")

		// Initialize messages with user message
		messages := []chat.Message{
			{
				Role:    chat.RoleUser,
				Content: userMessage,
			},
		}

		// Tool execution loop
		for iteration := 0; iteration < maxToolIterations; iteration++ {
			o.log.WithField("iteration", iteration).Debug("starting streaming tool iteration")
			// Build request
			req := &chat.Request{
				SystemPrompt: systemPrompt,
				Messages:     messages,
				Tools:        getToolDefinitions(),
				Documents:    docs,
			}

			// Get streaming response
			o.log.Debug("starting LLM stream")
			streamCh, err := o.chatClient.ChatStream(ctx, req)
			if err != nil {
				o.log.WithError(err).Error("failed to start LLM stream")
				eventCh <- OrchestratorStreamEvent{
					Type:  OrchestratorEventError,
					Error: fmt.Errorf("LLM stream failed: %w", err),
				}
				return
			}

			var contentBuilder strings.Builder
			var toolCalls []chat.ToolCall
			chunkCount := 0

			// Process stream chunks
			o.log.Debug("processing stream chunks")
			for chunk := range streamCh {
				chunkCount++
				if chunk.Error != nil {
					o.log.WithError(chunk.Error).Error("received error in stream chunk")
					eventCh <- OrchestratorStreamEvent{
						Type:  OrchestratorEventError,
						Error: chunk.Error,
					}
					return
				}

				if chunk.Content != "" {
					contentBuilder.WriteString(chunk.Content)
					eventCh <- OrchestratorStreamEvent{
						Type:    OrchestratorEventContent,
						Content: chunk.Content,
					}
				}

				if len(chunk.ToolCalls) > 0 {
					toolCalls = append(toolCalls, chunk.ToolCalls...)
				}
			}

			o.log.WithFields(logrus.Fields{
				"chunk_count":    chunkCount,
				"content_length": contentBuilder.Len(),
				"tool_calls":     len(toolCalls),
			}).Debug("finished processing stream chunks")

			// If no tool calls, we're done
			if len(toolCalls) == 0 {
				o.log.Debug("no tool calls, stream complete")
				return
			}

			// Add assistant message with tool calls
			messages = append(messages, chat.Message{
				Role:      chat.RoleAssistant,
				Content:   contentBuilder.String(),
				ToolCalls: toolCalls,
			})

			// Execute tool calls
			for _, toolCall := range toolCalls {
				o.log.WithFields(logrus.Fields{
					"tool":    toolCall.Name,
					"tool_id": toolCall.ID,
				}).Debug("executing streaming tool call")

				// Send tool start event
				eventCh <- OrchestratorStreamEvent{
					Type:        OrchestratorEventToolStart,
					ToolName:    toolCall.Name,
					Description: fmt.Sprintf("Executing %s...", toolCall.Name),
				}

				toolResult, err := o.executeTool(ctx, toolCall)

				success := err == nil
				if success {
					o.log.WithFields(logrus.Fields{
						"tool":          toolCall.Name,
						"result_length": len(toolResult),
					}).Debug("streaming tool execution succeeded")
				} else {
					o.log.WithError(err).WithField("tool", toolCall.Name).Warn("streaming tool execution failed")
				}

				eventCh <- OrchestratorStreamEvent{
					Type:        OrchestratorEventToolEnd,
					ToolName:    toolCall.Name,
					Description: fmt.Sprintf("Executed %s", toolCall.Name),
					Success:     success,
				}

				var resultContent string
				if err != nil {
					resultContent = fmt.Sprintf("Error executing tool: %s", err.Error())
				} else {
					resultContent = toolResult
				}

				// Add tool result message
				messages = append(messages, chat.Message{
					Role:       chat.RoleTool,
					Content:    resultContent,
					ToolCallID: toolCall.ID,
				})
			}
		}

		o.log.WithField("max_iterations", maxToolIterations).Error("streaming max tool iterations exceeded")
		eventCh <- OrchestratorStreamEvent{
			Type:  OrchestratorEventError,
			Error: fmt.Errorf("max tool iterations (%d) exceeded", maxToolIterations),
		}
	}()

	return eventCh, nil
}

func (o *Orchestrator) callLLMWithRetry(ctx context.Context, req *chat.Request) (*chat.Response, error) {
	var lastErr error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		o.log.WithField("attempt", attempt+1).Debug("attempting LLM call")
		resp, err := o.chatClient.Chat(ctx, req)
		if err == nil {
			o.log.WithField("attempt", attempt+1).Debug("LLM call succeeded")
			return resp, nil
		}

		lastErr = err
		o.log.WithError(err).WithField("attempt", attempt+1).Warn("LLM call failed, retrying")

		// Exponential backoff
		delay := retryBaseDelay * time.Duration(1<<attempt)
		o.log.WithField("delay_ms", delay.Milliseconds()).Debug("waiting before retry")
		select {
		case <-ctx.Done():
			o.log.Debug("context cancelled during retry wait")
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	o.log.WithField("attempts", retryAttempts).Error("all LLM retry attempts exhausted")
	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", retryAttempts, lastErr)
}

func (o *Orchestrator) executeTool(ctx context.Context, toolCall chat.ToolCall) (string, error) {
	switch toolCall.Name {
	case "query_nais_api":
		return o.executeGraphQLTool(ctx, toolCall.Arguments)
	default:
		return "", fmt.Errorf("unknown tool: %s", toolCall.Name)
	}
}

func (o *Orchestrator) executeGraphQLTool(ctx context.Context, args map[string]any) (string, error) {
	query, ok := args["query"].(string)
	if !ok {
		return "", fmt.Errorf("query argument is required and must be a string")
	}

	variables, _ := args["variables"].(map[string]any)

	o.log.WithFields(logrus.Fields{
		"query_length":  len(query),
		"has_variables": variables != nil,
	}).Debug("executing GraphQL tool")

	// Build GraphQL request
	gqlReq := map[string]any{
		"query": query,
	}
	if variables != nil {
		gqlReq["variables"] = variables
	}

	body, err := json.Marshal(gqlReq)
	if err != nil {
		return "", fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", o.naisAPIURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", o.authHeader)

	o.log.WithField("url", o.naisAPIURL+"/graphql").Debug("sending GraphQL request")

	// Execute request
	resp, err := o.httpClient.Do(req)
	if err != nil {
		o.log.WithError(err).Error("GraphQL request failed")
		return "", fmt.Errorf("GraphQL request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	o.log.WithFields(logrus.Fields{
		"status_code":     resp.StatusCode,
		"response_length": len(respBody),
	}).Debug("GraphQL response received")

	if resp.StatusCode != http.StatusOK {
		o.log.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"response":    string(respBody),
		}).Warn("GraphQL request returned non-OK status")
		return "", fmt.Errorf("GraphQL request returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return string(respBody), nil
}

func buildSystemPrompt(ctx *ChatContext, docs []rag.Document) string {
	var sb strings.Builder

	sb.WriteString(`You are a helpful assistant for the Nais platform, a Kubernetes-based application platform. You help users understand and troubleshoot their applications.

## Current Context
`)

	if ctx != nil {
		if ctx.Path != "" {
			sb.WriteString(fmt.Sprintf("- User is viewing: %s\n", ctx.Path))
		}
		if ctx.Team != "" {
			sb.WriteString(fmt.Sprintf("- Team: %s\n", ctx.Team))
		}
		if ctx.App != "" {
			sb.WriteString(fmt.Sprintf("- Application: %s\n", ctx.App))
		}
		if ctx.Env != "" {
			sb.WriteString(fmt.Sprintf("- Environment: %s\n", ctx.Env))
		}
	}

	sb.WriteString(`
## Available Tools
1. **query_nais_api**: Execute GraphQL queries to fetch real-time data about the user's teams, applications, deployments, and other resources.

## Guidelines
- For general questions about Nais features, use the documentation provided below.
- For specific questions about the user's resources, use the query_nais_api tool.
- Always provide actionable advice when possible.
- Include links to relevant documentation when helpful.
- If you encounter an error from a tool, explain it clearly to the user.
`)

	if len(docs) > 0 {
		sb.WriteString("\n## Documentation\n")
		for _, doc := range docs {
			sb.WriteString(fmt.Sprintf("\n### %s\n%s\nSource: %s\n", doc.Title, doc.Content, doc.URL))
		}
	}

	return sb.String()
}

func getToolDefinitions() []chat.ToolDefinition {
	return []chat.ToolDefinition{
		{
			Name: "query_nais_api",
			Description: `Execute a GraphQL query against the Nais API to fetch information about teams, applications, deployments, logs, and other resources. Use this for specific questions about the user's resources.

Example queries:
- Get team info: query { team(slug: "my-team") { slug purpose } }
- Get applications: query { team(slug: "my-team") { applications { nodes { name state } } } }
- Get app status: query { team(slug: "my-team") { applications(filter: {name: "my-app"}) { nodes { name state instances { nodes { name status { state message } } } } } } }`,
			Parameters: []chat.ParameterDefinition{
				{
					Name:        "query",
					Type:        "string",
					Description: "The GraphQL query to execute",
					Required:    true,
				},
				{
					Name:        "variables",
					Type:        "object",
					Description: "Variables for the GraphQL query (optional)",
					Required:    false,
				},
			},
		},
	}
}
