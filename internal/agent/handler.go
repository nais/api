// Package agent provides the AI chat service for the Nais platform.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/nais/api/internal/auth/authz"
	"github.com/sirupsen/logrus"
)

const (
	maxRAGResults = 5
)

// Handler implements the HTTP handler for the agent chat service.
type Handler struct {
	conversations  *ConversationStore
	chatClient     chat.StreamingClient
	ragClient      rag.DocumentSearcher
	mcpIntegration *MCPIntegration
	log            logrus.FieldLogger
}

// Config holds configuration for the agent handler.
type Config struct {
	Pool           *pgxpool.Pool
	ChatClient     chat.StreamingClient
	RAGClient      rag.DocumentSearcher
	GraphQLHandler *handler.Server
	TenantName     string
	Log            logrus.FieldLogger
}

// NewHandler creates a new agent HTTP handler.
func NewHandler(cfg Config) (*Handler, error) {
	// Create MCP integration for tool execution
	mcpIntegration, err := NewMCPIntegration(MCPIntegrationConfig{
		Handler:    cfg.GraphQLHandler,
		TenantName: cfg.TenantName,
		Log:        cfg.Log,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MCP integration: %w", err)
	}

	return &Handler{
		conversations:  NewConversationStore(cfg.Pool),
		chatClient:     cfg.ChatClient,
		ragClient:      cfg.RAGClient,
		mcpIntegration: mcpIntegration,
		log:            cfg.Log,
	}, nil
}

// RegisterRoutes registers the agent routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/chat", h.Chat)
	r.Post("/chat/stream", h.ChatStream)
	r.Get("/conversations", h.ListConversations)
	r.Get("/conversations/{conversationID}", h.GetConversation)
	r.Delete("/conversations/{conversationID}", h.DeleteConversation)
}

// ChatRequest represents a chat request.
type ChatRequest struct {
	Message        string       `json:"message"`
	ConversationID string       `json:"conversation_id,omitempty"`
	Context        *ChatContext `json:"context,omitempty"`
}

// ChatResponse represents a non-streaming chat response.
type ChatResponse struct {
	ConversationID string           `json:"conversation_id"`
	MessageID      string           `json:"message_id"`
	Content        string           `json:"content"`
	Blocks         []ContentBlock   `json:"blocks,omitempty"`
	Sources        []Source         `json:"sources,omitempty"`
	Usage          *chat.UsageStats `json:"usage,omitempty"`
}

// StreamEvent represents a server-sent event for streaming responses.
type StreamEvent struct {
	Type           string           `json:"type"`
	ConversationID string           `json:"conversation_id,omitempty"`
	MessageID      string           `json:"message_id,omitempty"`
	Content        string           `json:"content,omitempty"`
	Thinking       string           `json:"thinking,omitempty"`
	ToolName       string           `json:"tool_name,omitempty"`
	ToolSuccess    bool             `json:"tool_success,omitempty"`
	Description    string           `json:"description,omitempty"`
	Sources        []Source         `json:"sources,omitempty"`
	Chart          *ChartData       `json:"chart,omitempty"`
	Usage          *chat.UsageStats `json:"usage,omitempty"`
	ToolCallID     string           `json:"tool_call_id,omitempty"`
	ErrorCode      string           `json:"error_code,omitempty"`
	ErrorMessage   string           `json:"error_message,omitempty"`
}

// Chat handles non-streaming chat requests.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	log := h.log.WithField("method", "Chat")

	log.Debug("received chat request")

	// Parse request
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.WithError(err).Debug("failed to decode request body")
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.WithFields(logrus.Fields{
		"message_length":  len(req.Message),
		"conversation_id": req.ConversationID,
		"has_context":     req.Context != nil,
	}).Debug("parsed chat request")

	if strings.TrimSpace(req.Message) == "" {
		log.Debug("empty message in request")
		writeJSONError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		log.WithError(err).Debug("failed to get user from context")
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	log = log.WithField("user_id", userID)
	log.Debug("authenticated user")

	// Get or create conversation
	conversationID, err := h.conversations.GetOrCreateConversation(ctx, userID, req.ConversationID, req.Message)
	if err != nil {
		log.WithError(err).Error("failed to get or create conversation")
		writeJSONError(w, http.StatusInternalServerError, "failed to process conversation")
		return
	}

	log = log.WithField("conversation_id", conversationID)
	log.Debug("got/created conversation")

	// Load conversation history for context
	history, err := h.conversations.GetConversationHistory(ctx, conversationID)
	if err != nil {
		log.WithError(err).Warn("failed to load conversation history, continuing without")
		history = nil
	} else {
		log.WithField("history_length", len(history)).Debug("loaded conversation history")
	}

	// Perform RAG search
	log.Debug("performing RAG search")
	docs, sources, err := h.searchDocumentation(ctx, req.Message)
	if err != nil {
		log.WithError(err).Warn("RAG search failed, continuing without docs")
		RecordRAGSearch("error", 0, 0)
	} else {
		log.WithField("doc_count", len(docs)).Debug("RAG search completed")
		RecordRAGSearch("success", 0, len(docs))
	}

	// Build orchestrator and run conversation
	orchestrator := NewOrchestrator(
		h.chatClient,
		h.mcpIntegration,
		log,
	)

	log.Debug("starting orchestrator")
	result, err := orchestrator.Run(ctx, req.Message, req.Context, docs, conversationID, history)
	if err != nil {
		log.WithError(err).Error("orchestrator failed")
		writeJSONError(w, http.StatusInternalServerError, "failed to process chat request")
		return
	}

	log.WithFields(logrus.Fields{
		"block_count": len(result.Blocks),
	}).Debug("orchestrator completed")

	// Store messages
	if err := h.conversations.StoreMessages(ctx, conversationID, req.Message, result); err != nil {
		log.WithError(err).Error("failed to store messages")
		// Continue anyway - we have the response
	}

	RecordChatRequest("success", false, time.Since(start).Seconds(), result.Usage.InputTokens, result.Usage.OutputTokens)
	RecordToolIterations(countToolBlocks(result.Blocks))

	// Extract text content for source filtering
	textContent := extractTextFromBlocks(result.Blocks)

	// Filter sources to only include those actually referenced in the response
	usedSources := filterUsedSources(textContent, sources)

	// Build response
	resp := ChatResponse{
		ConversationID: conversationID.String(),
		MessageID:      uuid.New().String(),
		Content:        textContent,
		Blocks:         result.Blocks,
		Sources:        usedSources,
		Usage:          result.Usage,
	}

	log.Debug("sending chat response")
	writeJSON(w, http.StatusOK, resp)
}

// ChatStream handles streaming chat requests using Server-Sent Events.
func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	ctx := r.Context()
	log := h.log.WithField("method", "ChatStream")

	log.Debug("received streaming chat request")

	// Parse request
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.WithError(err).Debug("failed to decode request body")
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	log.WithFields(logrus.Fields{
		"message_length":  len(req.Message),
		"conversation_id": req.ConversationID,
		"has_context":     req.Context != nil,
	}).Debug("parsed streaming chat request")

	if strings.TrimSpace(req.Message) == "" {
		log.Debug("empty message in request")
		writeJSONError(w, http.StatusBadRequest, "message is required")
		return
	}

	// Get user ID from context
	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		log.WithError(err).Debug("failed to get user from context")
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	log = log.WithField("user_id", userID)
	log.Debug("authenticated user")

	// Get or create conversation
	conversationID, err := h.conversations.GetOrCreateConversation(ctx, userID, req.ConversationID, req.Message)
	if err != nil {
		log.WithError(err).Error("failed to get or create conversation")
		writeJSONError(w, http.StatusInternalServerError, "failed to process conversation")
		return
	}

	log = log.WithField("conversation_id", conversationID)
	log.Debug("got/created conversation")

	// Load conversation history for context
	history, err := h.conversations.GetConversationHistory(ctx, conversationID)
	if err != nil {
		log.WithError(err).Warn("failed to load conversation history, continuing without")
		history = nil
	} else {
		log.WithField("history_length", len(history)).Debug("loaded conversation history")
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Error("response writer does not support flushing")
		writeJSONError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	log.Debug("SSE stream initialized, sending metadata")

	// Send metadata event
	sendSSE(w, flusher, StreamEvent{
		Type:           "metadata",
		ConversationID: conversationID.String(),
		MessageID:      uuid.New().String(),
	})

	// Perform RAG search
	log.Debug("performing RAG search")
	docs, sources, err := h.searchDocumentation(ctx, req.Message)
	if err != nil {
		log.WithError(err).Warn("RAG search failed, continuing without docs")
		RecordRAGSearch("error", 0, 0)
	} else {
		log.WithField("doc_count", len(docs)).Debug("RAG search completed")
		RecordRAGSearch("success", 0, len(docs))
	}

	// Build orchestrator and run streaming conversation
	orchestrator := NewOrchestrator(
		h.chatClient,
		h.mcpIntegration,
		log,
	)

	log.Debug("starting streaming orchestrator")
	streamCh, err := orchestrator.RunStream(ctx, req.Message, req.Context, docs, conversationID, history)
	if err != nil {
		log.WithError(err).Error("orchestrator stream failed to start")
		sendSSE(w, flusher, StreamEvent{
			Type:         "error",
			ErrorCode:    "stream_error",
			ErrorMessage: "failed to start chat stream",
		})
		return
	}

	var fullContent strings.Builder
	var blocks []ContentBlock
	var currentThinking strings.Builder
	var totalUsage chat.UsageStats
	eventCount := 0

	log.Debug("processing stream events")
	for event := range streamCh {
		eventCount++
		switch event.Type {
		case OrchestratorEventToolStart:
			log.WithField("tool", event.ToolName).Debug("tool started")
			// Flush any accumulated content before tool starts
			if fullContent.Len() > 0 {
				blocks = append(blocks, ContentBlock{
					Type: ContentBlockTypeText,
					Text: fullContent.String(),
				})
				fullContent.Reset()
			}
			// Flush any accumulated thinking before tool starts
			if currentThinking.Len() > 0 {
				blocks = append(blocks, ContentBlock{
					Type:     ContentBlockTypeThinking,
					Thinking: currentThinking.String(),
				})
				currentThinking.Reset()
			}
			sendSSE(w, flusher, StreamEvent{
				Type:        "tool_start",
				ToolName:    event.ToolName,
				ToolCallID:  event.ToolCallID,
				Description: event.Description,
			})

		case OrchestratorEventToolEnd:
			log.WithFields(logrus.Fields{
				"tool":    event.ToolName,
				"success": event.Success,
			}).Debug("tool ended")
			sendSSE(w, flusher, StreamEvent{
				Type:        "tool_end",
				ToolName:    event.ToolName,
				ToolCallID:  event.ToolCallID,
				Description: event.Description,
				ToolSuccess: event.Success,
			})
			// Add tool use block with result for history reconstruction
			blocks = append(blocks, ContentBlock{
				Type:        ContentBlockTypeToolUse,
				ToolName:    event.ToolName,
				ToolCallID:  event.ToolCallID,
				ToolSuccess: event.Success,
				ToolResult:  event.ToolResult,
			})

		case OrchestratorEventThinking:
			log.Debug("received thinking content")
			currentThinking.WriteString(event.Thinking)
			sendSSE(w, flusher, StreamEvent{
				Type:     "thinking",
				Thinking: event.Thinking,
			})

		case OrchestratorEventChart:
			log.WithField("chart_title", event.Chart.Title).Debug("received chart data")
			// Add chart block
			blocks = append(blocks, ContentBlock{
				Type:  ContentBlockTypeChart,
				Chart: event.Chart,
			})
			sendSSE(w, flusher, StreamEvent{
				Type:  "chart",
				Chart: event.Chart,
			})

		case OrchestratorEventContent:
			fullContent.WriteString(event.Content)
			sendSSE(w, flusher, StreamEvent{
				Type:    "content",
				Content: event.Content,
			})

		case OrchestratorEventError:
			log.WithError(event.Error).Error("stream error event received")
			sendSSE(w, flusher, StreamEvent{
				Type:         "error",
				ErrorCode:    "stream_error",
				ErrorMessage: event.Error.Error(),
			})
			return

		case OrchestratorEventUsage:
			if event.Usage != nil {
				totalUsage.InputTokens += event.Usage.InputTokens
				totalUsage.OutputTokens += event.Usage.OutputTokens
				totalUsage.TotalTokens += event.Usage.TotalTokens
				if event.Usage.MaxTokens > 0 {
					totalUsage.MaxTokens = event.Usage.MaxTokens
				}
				sendSSE(w, flusher, StreamEvent{
					Type:  "usage",
					Usage: event.Usage,
				})
			}
		}
	}

	// Flush any remaining content
	if fullContent.Len() > 0 {
		blocks = append(blocks, ContentBlock{
			Type: ContentBlockTypeText,
			Text: fullContent.String(),
		})
	}
	// Flush any remaining thinking
	if currentThinking.Len() > 0 {
		blocks = append(blocks, ContentBlock{
			Type:     ContentBlockTypeThinking,
			Thinking: currentThinking.String(),
		})
	}

	log.WithFields(logrus.Fields{
		"event_count": eventCount,
		"block_count": len(blocks),
	}).Debug("stream processing completed")

	// Extract text content for source filtering
	textContent := extractTextFromBlocks(blocks)

	// Filter and send sources if available
	usedSources := filterUsedSources(textContent, sources)
	if len(usedSources) > 0 {
		log.WithField("source_count", len(usedSources)).Debug("sending sources")
		sendSSE(w, flusher, StreamEvent{
			Type:    "sources",
			Sources: usedSources,
		})
	}

	// Send done event
	log.Debug("sending done event")
	sendSSE(w, flusher, StreamEvent{
		Type: "done",
	})

	// Store messages asynchronously
	go func() {
		// Append usage block to ensure it's persisted
		blocks = append(blocks, ContentBlock{
			Type:  ContentBlockTypeUsage,
			Usage: &totalUsage,
		})

		result := &OrchestratorResult{
			Blocks: blocks,
			Usage:  &totalUsage,
		}
		if err := h.conversations.StoreMessages(context.Background(), conversationID, req.Message, result); err != nil {
			log.WithError(err).Error("failed to store messages")
		} else {
			log.Debug("messages stored successfully")
		}
	}()

	RecordChatRequest("success", true, time.Since(start).Seconds(), totalUsage.InputTokens, totalUsage.OutputTokens)
	RecordToolIterations(countToolBlocks(blocks))
	log.Debug("streaming chat request completed")
}

// ListConversations returns all conversations for the authenticated user.
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	conversations, err := h.conversations.ListConversations(ctx, userID)
	if err != nil {
		h.log.WithError(err).Error("failed to list conversations")
		writeJSONError(w, http.StatusInternalServerError, "failed to list conversations")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversations": conversations,
	})
}

// GetConversation returns a specific conversation with all its messages.
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	conversationIDStr := chi.URLParam(r, "conversationID")
	if conversationIDStr == "" {
		writeJSONError(w, http.StatusBadRequest, "conversation_id is required")
		return
	}

	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid conversation_id")
		return
	}

	conv, err := h.conversations.GetConversation(ctx, userID, conversationID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(w, http.StatusNotFound, "conversation not found")
			return
		}
		h.log.WithError(err).Error("failed to get conversation")
		writeJSONError(w, http.StatusInternalServerError, "failed to get conversation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"conversation": conv,
	})
}

// DeleteConversation deletes a conversation.
func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	conversationIDStr := chi.URLParam(r, "conversationID")
	if conversationIDStr == "" {
		writeJSONError(w, http.StatusBadRequest, "conversation_id is required")
		return
	}

	conversationID, err := uuid.Parse(conversationIDStr)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid conversation_id")
		return
	}

	if err := h.conversations.DeleteConversation(ctx, userID, conversationID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSONError(w, http.StatusNotFound, "conversation not found")
			return
		}
		h.log.WithError(err).Error("failed to delete conversation")
		writeJSONError(w, http.StatusInternalServerError, "failed to delete conversation")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
	})
}

func (h *Handler) searchDocumentation(ctx context.Context, query string) ([]rag.Document, []Source, error) {
	result, err := h.ragClient.Search(ctx, query, &rag.SearchOptions{
		MaxResults: maxRAGResults,
	})
	if err != nil {
		return nil, nil, err
	}

	// Deduplicate sources by URL (multiple chunks from same page should appear as one source)
	seen := make(map[string]bool)
	sources := make([]Source, 0, len(result.Documents))
	for _, doc := range result.Documents {
		if seen[doc.URL] {
			continue
		}
		seen[doc.URL] = true
		sources = append(sources, Source{
			Title: doc.Title,
			URL:   doc.URL,
		})
	}

	return result.Documents, sources, nil
}

// getUserIDFromContext extracts the user ID from the HTTP context.
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return uuid.Nil, fmt.Errorf("no user in context")
	}
	return actor.User.GetID(), nil
}

func sendSSE(w http.ResponseWriter, flusher http.Flusher, event StreamEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// filterUsedSources returns only the sources that appear to be referenced in the response.
// This ensures we don't return sources that weren't actually used by the LLM.
func filterUsedSources(response string, sources []Source) []Source {
	if len(sources) == 0 {
		return sources
	}

	responseLower := strings.ToLower(response)
	used := make([]Source, 0, len(sources))

	for _, source := range sources {
		// Check if the source title or URL is mentioned in the response
		titleLower := strings.ToLower(source.Title)
		urlLower := strings.ToLower(source.URL)

		// Check for title mention (with some flexibility for partial matches)
		titleMentioned := strings.Contains(responseLower, titleLower)

		// Check for URL mention
		urlMentioned := strings.Contains(responseLower, urlLower)

		// Also check for key parts of the title (e.g., "deployment" from "Nais Deployment Guide")
		// This helps catch cases where the LLM paraphrases the source name
		titleWords := strings.Fields(titleLower)
		keywordMentioned := false
		for _, word := range titleWords {
			// Skip common words that wouldn't indicate a specific reference
			if len(word) > 4 && word != "guide" && word != "documentation" && word != "nais" {
				if strings.Contains(responseLower, word) {
					keywordMentioned = true
					break
				}
			}
		}

		if titleMentioned || urlMentioned || keywordMentioned {
			used = append(used, source)
		}
	}

	return used
}

// extractTextFromBlocks extracts all text content from a slice of content blocks.
func extractTextFromBlocks(blocks []ContentBlock) string {
	var sb strings.Builder
	for _, block := range blocks {
		if block.Type == ContentBlockTypeText && block.Text != "" {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// countToolBlocks counts the number of tool use blocks.
func countToolBlocks(blocks []ContentBlock) int {
	count := 0
	for _, block := range blocks {
		if block.Type == ContentBlockTypeToolUse {
			count++
		}
	}
	return count
}
