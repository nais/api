package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/nais/api/internal/agent/tools"
	"github.com/sirupsen/logrus"
)

const (
	maxToolIterations  = 5
	requestTimeout     = 60 * time.Second
	retryAttempts      = 3
	retryBaseDelay     = 100 * time.Millisecond
	maxToolOutputChars = 50000 // Truncate tool outputs to prevent context window exhaustion
)

// OrchestratorEventType is the type of a streaming event from the orchestrator.
// It uses the same string values as the SSE event type field, so no translation is needed.
type OrchestratorEventType = string

const (
	OrchestratorEventToolStart OrchestratorEventType = "tool_start"
	OrchestratorEventToolEnd   OrchestratorEventType = "tool_end"
	OrchestratorEventContent   OrchestratorEventType = "content"
	OrchestratorEventThinking  OrchestratorEventType = "thinking"
	OrchestratorEventChart     OrchestratorEventType = "chart"
	OrchestratorEventError     OrchestratorEventType = "error"
	OrchestratorEventUsage     OrchestratorEventType = "usage"
	OrchestratorEventDone      OrchestratorEventType = "done"
)

// ChartData represents the data needed to render a chart on the client.
type ChartData = tools.ChartData

// ContentBlockType represents the type of content block in an assistant message.
type ContentBlockType string

const (
	// ContentBlockTypeThinking represents the model's reasoning/thought process.
	ContentBlockTypeThinking ContentBlockType = "thinking"
	// ContentBlockTypeText represents regular text output.
	ContentBlockTypeText ContentBlockType = "text"
	// ContentBlockTypeToolUse represents a tool invocation and its result.
	// Stored internally for LLM history reconstruction; filtered from client responses.
	ContentBlockTypeToolUse ContentBlockType = "tool_use"
	// ContentBlockTypeChart represents a chart to be rendered.
	ContentBlockTypeChart ContentBlockType = "chart"
)

// ContentBlock represents a single block of content in an assistant message.
// Messages are composed of multiple blocks displayed in order.
//
// Blocks of type "text", "thinking", and "chart" are shown to the client.
// Blocks of type "tool_use" are stored for LLM history reconstruction only
// and should be filtered out of client-visible responses.
type ContentBlock struct {
	// Type indicates the kind of content block.
	Type ContentBlockType `json:"type"`
	// Text is the text content (for "text" type blocks).
	Text string `json:"text,omitempty"`
	// Thinking is the model's reasoning (for "thinking" type blocks).
	Thinking string `json:"thinking,omitempty"`
	// ToolCallID is the unique identifier for the tool call (for "tool_use" type blocks).
	ToolCallID string `json:"tool_call_id,omitempty"`
	// ToolName is the name of the tool (for "tool_use" type blocks).
	ToolName string `json:"tool_name,omitempty"`
	// ToolSuccess indicates whether the tool execution succeeded (for "tool_use" type blocks).
	ToolSuccess bool `json:"tool_success,omitempty"`
	// ToolResult is the result returned by the tool (for "tool_use" type blocks).
	// Stored to reconstruct the full conversation history for subsequent LLM calls.
	ToolResult string `json:"tool_result,omitempty"`
	// Chart contains chart data (for "chart" type blocks).
	Chart *ChartData `json:"chart,omitempty"`
}

// OrchestratorStreamEvent represents an event in the streaming response from the orchestrator.
// Type matches the SSE wire format strings directly.
type OrchestratorStreamEvent struct {
	Type        OrchestratorEventType
	Content     string
	Thinking    string
	ToolName    string
	Description string
	Success     bool
	Error       error
	Chart       *ChartData
	Usage       *chat.UsageStats
	ToolCallID  string
	// Result is only set for OrchestratorEventDone and carries the complete
	// accumulated result (blocks + usage) for persistence.
	Result *OrchestratorResult
}

// ToStreamEvent converts an OrchestratorStreamEvent to a StreamEvent for SSE delivery.
// It handles all regular event types via direct field mapping. The caller is responsible
// for handling OrchestratorEventError and OrchestratorEventDone, which require control
// flow (return) and additional side effects (logging, storing messages, etc.).
func (e OrchestratorStreamEvent) ToStreamEvent() StreamEvent {
	return StreamEvent{
		Type:        e.Type,
		Content:     e.Content,
		Thinking:    e.Thinking,
		ToolName:    e.ToolName,
		ToolCallID:  e.ToolCallID,
		Description: e.Description,
		ToolSuccess: e.Success,
		Chart:       e.Chart,
		Usage:       e.Usage,
	}
}

// OrchestratorResult contains the complete result of an orchestrated conversation turn.
type OrchestratorResult struct {
	// Blocks contains the sequence of content blocks that make up the response.
	// Includes "text", "thinking", "chart", and internal "tool_use" blocks.
	Blocks []ContentBlock
	// Usage contains aggregated token usage statistics.
	Usage *chat.UsageStats
}

// Orchestrator manages the conversation loop between user, LLM, and tools.
type Orchestrator struct {
	chatClient chat.StreamingClient
	registry   *tools.Registry
	log        logrus.FieldLogger
}

// NewOrchestrator creates a new orchestrator.
func NewOrchestrator(
	chatClient chat.StreamingClient,
	registry *tools.Registry,
	log logrus.FieldLogger,
) *Orchestrator {
	return &Orchestrator{
		chatClient: chatClient,
		registry:   registry,
		log:        log,
	}
}

// conversationLoop manages the shared mutable state for a single conversation turn,
// used by both Run and RunStream.
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

func (o *Orchestrator) newConversationLoop(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	history []chat.Message,
) *conversationLoop {
	toolDefs := o.getToolDefinitions()
	systemPrompt := o.buildSystemPrompt(chatCtx, docs, toolDefs)

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
		tools:        toolDefs,
		systemPrompt: systemPrompt,
		docs:         docs,
		log:          o.log,
	}
}

func (cl *conversationLoop) buildRequest() *chat.Request {
	return &chat.Request{
		SystemPrompt: cl.systemPrompt,
		Messages:     cl.messages,
		Tools:        cl.tools,
		Documents:    cl.docs,
	}
}

func (cl *conversationLoop) addThinkingBlock(thinking string) {
	if thinking != "" {
		cl.blocks = append(cl.blocks, ContentBlock{
			Type:     ContentBlockTypeThinking,
			Thinking: thinking,
		})
	}
}

func (cl *conversationLoop) addTextBlock(text string) {
	if text != "" {
		cl.blocks = append(cl.blocks, ContentBlock{
			Type: ContentBlockTypeText,
			Text: text,
		})
	}
}

func (cl *conversationLoop) addAssistantMessage(content string, toolCalls []chat.ToolCall) {
	cl.messages = append(cl.messages, chat.Message{
		Role:      chat.RoleAssistant,
		Content:   content,
		ToolCalls: toolCalls,
	})
}

// toolExecutionResult holds the result of executing a single tool call.
type toolExecutionResult struct {
	ToolCall  chat.ToolCall
	Result    string
	ChartData *ChartData
	Success   bool
	Error     error
}

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

// recordToolExecution stores a tool execution in the block history and appends the
// tool result message for the next LLM iteration.
func (cl *conversationLoop) recordToolExecution(result toolExecutionResult) {
	cl.blocks = append(cl.blocks, ContentBlock{
		Type:        ContentBlockTypeToolUse,
		ToolCallID:  result.ToolCall.ID,
		ToolName:    result.ToolCall.Name,
		ToolSuccess: result.Success,
		ToolResult:  result.Result,
	})

	if result.ChartData != nil {
		cl.blocks = append(cl.blocks, ContentBlock{
			Type:  ContentBlockTypeChart,
			Chart: result.ChartData,
		})
	}

	cl.messages = append(cl.messages, chat.Message{
		Role:       chat.RoleTool,
		Content:    result.Result,
		ToolCallID: result.ToolCall.ID,
	})
}

func (cl *conversationLoop) accumulateUsage(usage *chat.UsageStats) {
	if usage != nil {
		cl.totalUsage.InputTokens += usage.InputTokens
		cl.totalUsage.OutputTokens += usage.OutputTokens
		cl.totalUsage.TotalTokens += usage.TotalTokens
		if usage.MaxTokens > 0 {
			cl.totalUsage.MaxTokens = usage.MaxTokens
		}
	}
}

func (cl *conversationLoop) result() *OrchestratorResult {
	return &OrchestratorResult{
		Blocks: cl.blocks,
		Usage:  &cl.totalUsage,
	}
}

// Run executes a non-streaming conversation turn and returns the complete result.
func (o *Orchestrator) Run(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	history []chat.Message,
) (*OrchestratorResult, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	o.log.WithFields(logrus.Fields{
		"message_length": len(userMessage),
		"doc_count":      len(docs),
		"history_length": len(history),
	}).Debug("starting orchestrator run")

	loop := o.newConversationLoop(ctx, userMessage, chatCtx, docs, history)

	for iteration := 0; iteration < maxToolIterations; iteration++ {
		o.log.WithField("iteration", iteration).Debug("starting tool iteration")

		resp, err := o.callLLMWithRetry(ctx, loop.buildRequest())
		if err != nil {
			return nil, fmt.Errorf("LLM call failed: %w", err)
		}

		loop.accumulateUsage(resp.Usage)

		if len(resp.ToolCalls) == 0 {
			loop.addThinkingBlock(resp.Thinking)
			loop.addTextBlock(resp.Content)
			return loop.result(), nil
		}

		loop.addThinkingBlock(resp.Thinking)
		loop.addTextBlock(resp.Content)
		loop.addAssistantMessage(resp.Content, resp.ToolCalls)

		for _, toolCall := range resp.ToolCalls {
			result := loop.executeToolCall(toolCall)
			loop.recordToolExecution(result)
		}
	}

	return nil, fmt.Errorf("max tool iterations (%d) exceeded", maxToolIterations)
}

// RunStream executes a streaming conversation turn. Events are sent to the returned channel.
// The final event is always OrchestratorEventDone, which carries the complete OrchestratorResult
// for persistence. The channel is closed after the Done event is sent.
func (o *Orchestrator) RunStream(
	ctx context.Context,
	userMessage string,
	chatCtx *ChatContext,
	docs []rag.Document,
	history []chat.Message,
) <-chan OrchestratorStreamEvent {
	eventCh := make(chan OrchestratorStreamEvent, 100)

	go func() {
		defer close(eventCh)

		ctx, cancel := context.WithTimeout(ctx, requestTimeout)
		defer cancel()

		loop := o.newConversationLoop(ctx, userMessage, chatCtx, docs, history)

		for iteration := 0; iteration < maxToolIterations; iteration++ {
			o.log.WithField("iteration", iteration).Debug("starting streaming tool iteration")

			streamCh, err := o.chatClient.ChatStream(ctx, loop.buildRequest())
			if err != nil {
				eventCh <- OrchestratorStreamEvent{
					Type:  OrchestratorEventError,
					Error: fmt.Errorf("LLM stream failed: %w", err),
				}
				return
			}

			content, thinking, toolCalls, usage, abort := o.processStream(streamCh, eventCh)
			if abort {
				return
			}

			loop.accumulateUsage(usage)

			if len(toolCalls) == 0 {
				// No tool calls — this is the final response.
				loop.addThinkingBlock(thinking)
				loop.addTextBlock(content)
				result := loop.result()
				eventCh <- OrchestratorStreamEvent{
					Type:   OrchestratorEventDone,
					Usage:  result.Usage,
					Result: result,
				}
				return
			}

			// Flush accumulated content/thinking to blocks before executing tools.
			loop.addThinkingBlock(thinking)
			loop.addTextBlock(content)
			loop.addAssistantMessage(content, toolCalls)

			o.executeToolCallsStreaming(loop, toolCalls, eventCh)
		}

		eventCh <- OrchestratorStreamEvent{
			Type:  OrchestratorEventError,
			Error: fmt.Errorf("max tool iterations (%d) exceeded", maxToolIterations),
		}
	}()

	return eventCh
}

// processStream consumes the LLM stream, forwarding events and accumulating results.
// Returns accumulated content, thinking, tool calls, usage, and whether to abort (due to error).
func (o *Orchestrator) processStream(
	streamCh <-chan chat.StreamChunk,
	eventCh chan<- OrchestratorStreamEvent,
) (content string, thinking string, toolCalls []chat.ToolCall, usage *chat.UsageStats, abort bool) {
	var contentBuilder strings.Builder
	var thinkingBuilder strings.Builder
	usage = &chat.UsageStats{}

	for chunk := range streamCh {
		if chunk.Error != nil {
			o.log.WithError(chunk.Error).Error("received error in stream chunk")
			eventCh <- OrchestratorStreamEvent{
				Type:  OrchestratorEventError,
				Error: chunk.Error,
			}
			return "", "", nil, nil, true
		}

		if chunk.Usage != nil {
			usage.InputTokens += chunk.Usage.InputTokens
			usage.OutputTokens += chunk.Usage.OutputTokens
			usage.TotalTokens += chunk.Usage.TotalTokens
			if chunk.Usage.MaxTokens > 0 {
				usage.MaxTokens = chunk.Usage.MaxTokens
			}
		}

		if chunk.Thinking != "" {
			thinkingBuilder.WriteString(chunk.Thinking)
			eventCh <- OrchestratorStreamEvent{
				Type:     OrchestratorEventThinking,
				Thinking: chunk.Thinking,
			}
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

	return contentBuilder.String(), thinkingBuilder.String(), toolCalls, usage, false
}

// executeToolCallsStreaming executes tool calls, emits streaming events, and records results
// in the conversation loop for the next LLM iteration.
func (o *Orchestrator) executeToolCallsStreaming(
	loop *conversationLoop,
	toolCalls []chat.ToolCall,
	eventCh chan<- OrchestratorStreamEvent,
) {
	for _, toolCall := range toolCalls {
		eventCh <- OrchestratorStreamEvent{
			Type:        OrchestratorEventToolStart,
			ToolName:    toolCall.Name,
			ToolCallID:  toolCall.ID,
			Description: fmt.Sprintf("Executing %s...", toolCall.Name),
		}

		result := loop.executeToolCall(toolCall)

		eventCh <- OrchestratorStreamEvent{
			Type:        OrchestratorEventToolEnd,
			ToolName:    toolCall.Name,
			ToolCallID:  toolCall.ID,
			Description: fmt.Sprintf("Executed %s", toolCall.Name),
			Success:     result.Success,
		}

		if result.ChartData != nil {
			eventCh <- OrchestratorStreamEvent{
				Type:  OrchestratorEventChart,
				Chart: result.ChartData,
			}
		}

		// Record in the loop (adds tool_use block + tool result message for next iteration).
		loop.recordToolExecution(result)
	}
}

func (o *Orchestrator) callLLMWithRetry(ctx context.Context, req *chat.Request) (*chat.Response, error) {
	var lastErr error
	for attempt := 0; attempt < retryAttempts; attempt++ {
		resp, err := o.chatClient.Chat(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		o.log.WithError(err).WithField("attempt", attempt+1).Warn("LLM call failed, retrying")

		delay := retryBaseDelay * time.Duration(1<<attempt)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, fmt.Errorf("LLM call failed after %d attempts: %w", retryAttempts, lastErr)
}
