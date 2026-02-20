package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/sirupsen/logrus"
)

const (
	maxToolIterations  = 5
	requestTimeout     = 60 * time.Second
	retryAttempts      = 3
	retryBaseDelay     = 100 * time.Millisecond
	maxToolOutputChars = 50000 // Truncate tool outputs to prevent context window exhaustion

	// Tool name for rendering charts
	renderChartToolName = "render_chart"
)

// OrchestratorEventType represents the type of streaming event from the orchestrator.
type OrchestratorEventType int

const (
	OrchestratorEventToolStart OrchestratorEventType = iota
	OrchestratorEventToolEnd
	OrchestratorEventContent
	OrchestratorEventThinking // Model's reasoning/thought process (when thinking mode is enabled)
	OrchestratorEventChart    // Chart data to be rendered by the client
	OrchestratorEventError
	OrchestratorEventUsage // Token usage statistics
)

// ChartData represents the data needed to render a chart on the client.
type ChartData struct {
	// ChartType is the type of chart (currently only "line" is supported)
	ChartType string `json:"chart_type"`
	// Title is a human-readable title for the chart
	Title string `json:"title"`
	// Environment is the environment to query metrics from
	Environment string `json:"environment"`
	// Query is the Prometheus query to execute
	Query string `json:"query"`
	// Interval is the time interval for the query (1h, 6h, 1d, 7d, 30d)
	Interval string `json:"interval,omitempty"`
	// YFormat is the format type for Y-axis values (number, percentage, bytes, cpu_cores, duration)
	YFormat string `json:"y_format,omitempty"`
	// LabelTemplate is a template string for formatting labels, e.g., "{pod}/{container}"
	LabelTemplate string `json:"label_template,omitempty"`
}

// ContentBlockType represents the type of content block in an assistant message.
type ContentBlockType string

const (
	// ContentBlockTypeThinking represents the model's reasoning/thought process.
	ContentBlockTypeThinking ContentBlockType = "thinking"
	// ContentBlockTypeText represents regular text output.
	ContentBlockTypeText ContentBlockType = "text"
	// ContentBlockTypeToolUse represents a tool invocation.
	ContentBlockTypeToolUse ContentBlockType = "tool_use"
	// ContentBlockTypeChart represents a chart to be rendered.
	ContentBlockTypeChart ContentBlockType = "chart"
	// ContentBlockTypeUsage represents usage statistics.
	ContentBlockTypeUsage ContentBlockType = "usage"
)

// ContentBlock represents a single block of content in an assistant message.
// Messages are composed of multiple blocks that are displayed in order.
type ContentBlock struct {
	// Type indicates the kind of content block.
	Type ContentBlockType `json:"type"`
	// Text is the text content (for "text" type blocks).
	Text string `json:"text,omitempty"`
	// Thinking is the model's reasoning (for "thinking" type blocks).
	Thinking string `json:"thinking,omitempty"`
	// ToolCallID is the unique identifier for the tool call (for "tool_use" type blocks).
	// This is important for providers that use unique IDs (like OpenAI's call_... IDs).
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName is the name of the tool (for "tool_use" type blocks).
	ToolName string `json:"tool_name,omitempty"`
	// ToolSuccess indicates whether the tool execution succeeded (for "tool_use" type blocks).
	ToolSuccess bool `json:"tool_success,omitempty"`
	// ToolResult is the result returned by the tool (for "tool_use" type blocks).
	// This is stored to reconstruct the full conversation history for subsequent LLM calls.
	ToolResult string `json:"tool_result,omitempty"`
	// Chart contains chart data (for "chart" type blocks).
	Chart *ChartData `json:"chart,omitempty"`
	// Usage contains usage statistics (for "usage" type blocks).
	Usage *chat.UsageStats `json:"usage,omitempty"`
}

// OrchestratorStreamEvent represents an event in the streaming response from the orchestrator.
type OrchestratorStreamEvent struct {
	Type        OrchestratorEventType
	Content     string
	Thinking    string // Model's reasoning (only set for OrchestratorEventThinking)
	ToolName    string
	Description string
	Success     bool
	Error       error
	Chart       *ChartData       // Chart data (only set for OrchestratorEventChart)
	Usage       *chat.UsageStats // Usage statistics (only set for OrchestratorEventUsage)
	ToolCallID  string           // Unique identifier for the tool call
	ToolResult  string           // Result from tool execution (only set for OrchestratorEventToolEnd)
}

// OrchestratorResult contains the result of an orchestrated conversation.
type OrchestratorResult struct {
	// Blocks contains the sequence of content blocks that make up the response.
	Blocks []ContentBlock
	// Usage contains token usage statistics.
	Usage *chat.UsageStats
}

// Orchestrator manages the conversation loop between user, LLM, and tools.
type Orchestrator struct {
	chatClient      chat.StreamingClient
	toolIntegration *ToolIntegration
	log             logrus.FieldLogger
}

// NewOrchestrator creates a new orchestrator with tool integration.
func NewOrchestrator(
	chatClient chat.StreamingClient,
	toolIntegration *ToolIntegration,
	log logrus.FieldLogger,
) *Orchestrator {
	return &Orchestrator{
		chatClient:      chatClient,
		toolIntegration: toolIntegration,
		log:             log,
	}
}

// conversationLoop manages the shared state for a conversation turn loop.
// This abstraction reduces duplication between Run and RunStream.
type conversationLoop struct {
	orchestrator *Orchestrator
	ctx          context.Context
	messages     []chat.Message
	tools        []chat.ToolDefinition
	systemPrompt string
	docs         []rag.Document
	blocks       []ContentBlock
	totalUsage   chat.UsageStats
	log          logrus.FieldLogger
}

// newConversationLoop creates a new conversation loop with initialized state.
func (o *Orchestrator) newConversationLoop(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	history []chat.Message,
) *conversationLoop {
	tools := o.getToolDefinitions()
	systemPrompt := o.buildSystemPrompt(chatCtx, docs, tools)

	// Initialize messages with conversation history plus current user message
	messages := make([]chat.Message, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, chat.Message{
		Role:    chat.RoleUser,
		Content: userMessage,
	})

	return &conversationLoop{
		orchestrator: o,
		ctx:          ctx,
		messages:     messages,
		tools:        tools,
		systemPrompt: systemPrompt,
		docs:         docs,
		blocks:       nil,
		log:          o.log,
	}
}

// buildRequest creates a chat request from the current state.
func (cl *conversationLoop) buildRequest() *chat.Request {
	return &chat.Request{
		SystemPrompt: cl.systemPrompt,
		Messages:     cl.messages,
		Tools:        cl.tools,
		Documents:    cl.docs,
	}
}

// addThinkingBlock adds a thinking block if the content is non-empty.
func (cl *conversationLoop) addThinkingBlock(thinking string) {
	if thinking != "" {
		cl.blocks = append(cl.blocks, ContentBlock{
			Type:     ContentBlockTypeThinking,
			Thinking: thinking,
		})
	}
}

// addTextBlock adds a text block if the content is non-empty.
func (cl *conversationLoop) addTextBlock(text string) {
	if text != "" {
		cl.blocks = append(cl.blocks, ContentBlock{
			Type: ContentBlockTypeText,
			Text: text,
		})
	}
}

// addAssistantMessage adds an assistant message with tool calls to the conversation.
func (cl *conversationLoop) addAssistantMessage(content string, toolCalls []chat.ToolCall) {
	cl.messages = append(cl.messages, chat.Message{
		Role:      chat.RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	})
}

// toolExecutionResult holds the result of executing a single tool.
type toolExecutionResult struct {
	ToolCall  chat.ToolCall
	Result    string
	ChartData *ChartData
	Success   bool
	Error     error
}

// executeToolCall executes a single tool call and returns the result.
func (cl *conversationLoop) executeToolCall(toolCall chat.ToolCall) toolExecutionResult {
	cl.log.WithFields(logrus.Fields{
		"tool":    toolCall.Name,
		"tool_id": toolCall.ID,
	}).Debug("executing tool call")

	toolResult, chartData, err := cl.orchestrator.executeTool(cl.ctx, toolCall)
	success := err == nil

	var resultContent string
	if err != nil {
		resultContent = fmt.Sprintf("Error executing tool: %s", err.Error())
		cl.log.WithError(err).WithField("tool", toolCall.Name).Warn("tool execution failed")
	} else {
		cl.log.WithFields(logrus.Fields{
			"tool":          toolCall.Name,
			"result_length": len(toolResult),
			"has_chart":     chartData != nil,
		}).Debug("tool execution succeeded")
		resultContent = toolResult
	}

	return toolExecutionResult{
		ToolCall:  toolCall,
		Result:    resultContent,
		ChartData: chartData,
		Success:   success,
		Error:     err,
	}
}

// recordToolExecution records a tool execution in blocks and messages.
func (cl *conversationLoop) recordToolExecution(result toolExecutionResult) {
	// Add tool use block with result for history reconstruction
	cl.blocks = append(cl.blocks, ContentBlock{
		Type:        ContentBlockTypeToolUse,
		ToolCallID:  result.ToolCall.ID,
		ToolName:    result.ToolCall.Name,
		ToolSuccess: result.Success,
		ToolResult:  result.Result,
	})

	// Add chart block if this was a chart tool
	if result.ChartData != nil {
		cl.blocks = append(cl.blocks, ContentBlock{
			Type:  ContentBlockTypeChart,
			Chart: result.ChartData,
		})
	}

	// Add tool result message
	cl.messages = append(cl.messages, chat.Message{
		Role:       chat.RoleTool,
		Content:    result.Result,
		ToolCallID: result.ToolCall.ID,
	})
}

// accumulateUsage adds usage stats to the total.
func (cl *conversationLoop) accumulateUsage(usage *chat.UsageStats) {
	if usage != nil {
		cl.totalUsage.InputTokens += usage.InputTokens
		cl.totalUsage.OutputTokens += usage.OutputTokens
		cl.totalUsage.TotalTokens += usage.TotalTokens
		if usage.MaxTokens > 0 {
			cl.totalUsage.MaxTokens = usage.MaxTokens
		}
		cl.log.WithFields(logrus.Fields{
			"input_tokens":  usage.InputTokens,
			"output_tokens": usage.OutputTokens,
			"total_tokens":  usage.TotalTokens,
			"max_tokens":    usage.MaxTokens,
		}).Debug("LLM usage stats")
	}
}

// result returns the final orchestrator result.
func (cl *conversationLoop) result() *OrchestratorResult {
	// Append usage block to ensure it's persisted
	cl.blocks = append(cl.blocks, ContentBlock{
		Type:  ContentBlockTypeUsage,
		Usage: &cl.totalUsage,
	})

	return &OrchestratorResult{
		Blocks: cl.blocks,
		Usage:  &cl.totalUsage,
	}
}

// Run executes a non-streaming conversation.
func (o *Orchestrator) Run(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	conversationID uuid.UUID,
	history []chat.Message,
) (*OrchestratorResult, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	o.log.WithFields(logrus.Fields{
		"conversation_id": conversationID,
		"message_length":  len(userMessage),
		"doc_count":       len(docs),
		"history_length":  len(history),
	}).Debug("starting orchestrator run")

	loop := o.newConversationLoop(ctx, userMessage, chatCtx, docs, history)
	o.log.WithField("prompt_length", len(loop.systemPrompt)).Debug("built system prompt")

	// Tool execution loop
	for iteration := 0; iteration < maxToolIterations; iteration++ {
		o.log.WithField("iteration", iteration).Debug("starting tool iteration")

		// Call LLM with retries
		resp, err := o.callLLMWithRetry(ctx, loop.buildRequest())
		if err != nil {
			o.log.WithError(err).Error("LLM call failed")
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		loop.accumulateUsage(resp.Usage)

		o.log.WithFields(logrus.Fields{
			"content_length": len(resp.Content),
			"tool_calls":     len(resp.ToolCalls),
		}).Debug("LLM response received")

		// If no tool calls, we're done - add final blocks and return
		if len(resp.ToolCalls) == 0 {
			o.log.Debug("no tool calls, returning response")
			loop.addThinkingBlock(resp.Thinking)
			loop.addTextBlock(resp.Content)
			return loop.result(), nil
		}

		// Process response with tool calls
		loop.addThinkingBlock(resp.Thinking)
		loop.addTextBlock(resp.Content)
		loop.addAssistantMessage(resp.Content, resp.ToolCalls)

		// Execute all tool calls
		for _, toolCall := range resp.ToolCalls {
			result := loop.executeToolCall(toolCall)
			loop.recordToolExecution(result)
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
	history []chat.Message,
) (<-chan OrchestratorStreamEvent, error) {
	eventCh := make(chan OrchestratorStreamEvent, 100)

	o.log.WithFields(logrus.Fields{
		"conversation_id": conversationID,
		"message_length":  len(userMessage),
		"doc_count":       len(docs),
		"history_length":  len(history),
	}).Debug("starting streaming orchestrator")

	go func() {
		defer close(eventCh)

		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		loop := o.newConversationLoop(ctx, userMessage, chatCtx, docs, history)
		o.log.WithField("prompt_length", len(loop.systemPrompt)).Debug("built system prompt")

		// Tool execution loop
		for iteration := 0; iteration < maxToolIterations; iteration++ {
			o.log.WithField("iteration", iteration).Debug("starting streaming tool iteration")

			// Get streaming response
			streamCh, err := o.chatClient.ChatStream(ctx, loop.buildRequest())
			if err != nil {
				o.log.WithError(err).Error("failed to start LLM stream")
				eventCh <- OrchestratorStreamEvent{
					Type:  OrchestratorEventError,
					Error: fmt.Errorf("LLM stream failed: %w", err),
				}
				return
			}

			// Process stream and collect results
			content, toolCalls, usage, done := o.processStream(streamCh, eventCh)
			if done {
				return // Error occurred or context cancelled
			}

			loop.accumulateUsage(usage)

			// If no tool calls, we're done
			if len(toolCalls) == 0 {
				eventCh <- OrchestratorStreamEvent{
					Type:  OrchestratorEventUsage,
					Usage: &loop.totalUsage,
				}
				o.log.Debug("no tool calls, stream complete")
				return
			}

			// Add assistant message for tool calls
			loop.addAssistantMessage(content, toolCalls)

			// Execute tool calls with streaming events
			o.executeToolCallsStreaming(loop, toolCalls, eventCh)
		}

		o.log.WithField("max_iterations", maxToolIterations).Error("streaming max tool iterations exceeded")
		eventCh <- OrchestratorStreamEvent{
			Type:  OrchestratorEventError,
			Error: fmt.Errorf("max tool iterations (%d) exceeded", maxToolIterations),
		}
	}()

	return eventCh, nil
}

// processStream consumes the LLM stream, emitting events and collecting results.
// Returns the accumulated content, tool calls, and whether to stop (due to error).
func (o *Orchestrator) processStream(
	streamCh <-chan chat.StreamChunk,
	eventCh chan<- OrchestratorStreamEvent,
) (content string, toolCalls []chat.ToolCall, usage *chat.UsageStats, done bool) {
	var contentBuilder strings.Builder
	chunkCount := 0
	usage = &chat.UsageStats{}

	o.log.Debug("processing stream chunks")
	for chunk := range streamCh {
		chunkCount++

		if chunk.Error != nil {
			o.log.WithError(chunk.Error).Error("received error in stream chunk")
			eventCh <- OrchestratorStreamEvent{
				Type:  OrchestratorEventError,
				Error: chunk.Error,
			}
			return "", nil, nil, true
		}

		if chunk.Usage != nil {
			usage.InputTokens += chunk.Usage.InputTokens
			usage.OutputTokens += chunk.Usage.OutputTokens
			usage.TotalTokens += chunk.Usage.TotalTokens
			if chunk.Usage.MaxTokens > 0 {
				usage.MaxTokens = chunk.Usage.MaxTokens
			}
		}

		// Emit thinking content if present
		if chunk.Thinking != "" {
			eventCh <- OrchestratorStreamEvent{
				Type:     OrchestratorEventThinking,
				Thinking: chunk.Thinking,
			}
		}

		// Emit and accumulate content
		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
			eventCh <- OrchestratorStreamEvent{
				Type:    OrchestratorEventContent,
				Content: chunk.Content,
			}
		}

		// Collect tool calls
		if len(chunk.ToolCalls) > 0 {
			toolCalls = append(toolCalls, chunk.ToolCalls...)
		}
	}

	o.log.WithFields(logrus.Fields{
		"chunk_count":    chunkCount,
		"content_length": contentBuilder.Len(),
		"tool_calls":     len(toolCalls),
	}).Debug("finished processing stream chunks")

	return contentBuilder.String(), toolCalls, usage, false
}

// executeToolCallsStreaming executes tool calls and emits streaming events.
func (o *Orchestrator) executeToolCallsStreaming(
	loop *conversationLoop,
	toolCalls []chat.ToolCall,
	eventCh chan<- OrchestratorStreamEvent,
) {
	for _, toolCall := range toolCalls {
		// Emit tool start event
		eventCh <- OrchestratorStreamEvent{
			Type:        OrchestratorEventToolStart,
			ToolName:    toolCall.Name,
			ToolCallID:  toolCall.ID,
			Description: fmt.Sprintf("Executing %s...", toolCall.Name),
		}

		// Execute the tool
		result := loop.executeToolCall(toolCall)

		// Emit tool end event with result for history reconstruction
		eventCh <- OrchestratorStreamEvent{
			Type:        OrchestratorEventToolEnd,
			ToolName:    toolCall.Name,
			ToolCallID:  toolCall.ID,
			Description: fmt.Sprintf("Executed %s", toolCall.Name),
			Success:     result.Success,
			ToolResult:  result.Result,
		}

		// Emit chart event if applicable
		if result.ChartData != nil {
			eventCh <- OrchestratorStreamEvent{
				Type:  OrchestratorEventChart,
				Chart: result.ChartData,
			}
		}

		// Record in conversation (for next iteration)
		loop.messages = append(loop.messages, chat.Message{
			Role:       chat.RoleTool,
			Content:    result.Result,
			ToolCallID: result.ToolCall.ID,
		})
	}
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
