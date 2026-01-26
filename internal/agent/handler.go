// Package agent provides the AI chat service for the Nais platform.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/nais/api/internal/auth/authz"
	"github.com/sirupsen/logrus"
)

var _ = logrus.Fields{} // ensure logrus is used

const (
	maxRAGResults = 5
)

// Handler implements the HTTP handler for the agent chat service.
type Handler struct {
	conversations *ConversationStore
	chatClient    chat.StreamingClient
	ragClient     rag.DocumentSearcher
	naisAPIURL    string
	log           logrus.FieldLogger
}

// Config holds configuration for the agent handler.
type Config struct {
	Pool       *pgxpool.Pool
	ChatClient chat.StreamingClient
	RAGClient  rag.DocumentSearcher
	NaisAPIURL string
	Log        logrus.FieldLogger
}

// NewHandler creates a new agent HTTP handler.
func NewHandler(cfg Config) *Handler {
	return &Handler{
		conversations: NewConversationStore(cfg.Pool),
		chatClient:    cfg.ChatClient,
		ragClient:     cfg.RAGClient,
		naisAPIURL:    cfg.NaisAPIURL,
		log:           cfg.Log,
	}
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
	ConversationID string     `json:"conversation_id"`
	MessageID      string     `json:"message_id"`
	Content        string     `json:"content"`
	ToolsUsed      []ToolUsed `json:"tools_used,omitempty"`
	Sources        []Source   `json:"sources,omitempty"`
}

// StreamEvent represents a server-sent event for streaming responses.
type StreamEvent struct {
	Type           string   `json:"type"`
	ConversationID string   `json:"conversation_id,omitempty"`
	MessageID      string   `json:"message_id,omitempty"`
	Content        string   `json:"content,omitempty"`
	ToolName       string   `json:"tool_name,omitempty"`
	ToolSuccess    bool     `json:"tool_success,omitempty"`
	Description    string   `json:"description,omitempty"`
	Sources        []Source `json:"sources,omitempty"`
	ErrorCode      string   `json:"error_code,omitempty"`
	ErrorMessage   string   `json:"error_message,omitempty"`
}

// Chat handles non-streaming chat requests.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
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

	// Get authorization header
	authHeader := r.Header.Get("Authorization")
	log.WithField("has_auth_header", authHeader != "").Debug("checked auth header")

	// Build orchestrator and run conversation
	orchestrator := NewOrchestrator(
		h.chatClient,
		h.naisAPIURL,
		authHeader,
		log,
	)

	log.Debug("starting orchestrator")
	result, err := orchestrator.Run(ctx, req.Message, req.Context, docs, conversationID)
	if err != nil {
		log.WithError(err).Error("orchestrator failed")
		writeJSONError(w, http.StatusInternalServerError, "failed to process chat request")
		return
	}

	log.WithFields(logrus.Fields{
		"content_length": len(result.Content),
		"tools_used":     len(result.ToolsUsed),
	}).Debug("orchestrator completed")

	// Store messages
	if err := h.conversations.StoreMessages(ctx, conversationID, req.Message, result); err != nil {
		log.WithError(err).Error("failed to store messages")
		// Continue anyway - we have the response
	}

	RecordChatRequest("success", false, 0)
	RecordToolIterations(len(result.ToolsUsed))

	// Build response
	resp := ChatResponse{
		ConversationID: conversationID.String(),
		MessageID:      uuid.New().String(),
		Content:        result.Content,
		ToolsUsed:      result.ToolsUsed,
		Sources:        sources,
	}

	log.Debug("sending chat response")
	writeJSON(w, http.StatusOK, resp)
}

// ChatStream handles streaming chat requests using Server-Sent Events.
func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
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

	// Get authorization header
	authHeader := r.Header.Get("Authorization")
	log.WithField("has_auth_header", authHeader != "").Debug("checked auth header")

	// Build orchestrator and run streaming conversation
	orchestrator := NewOrchestrator(
		h.chatClient,
		h.naisAPIURL,
		authHeader,
		log,
	)

	log.Debug("starting streaming orchestrator")
	streamCh, err := orchestrator.RunStream(ctx, req.Message, req.Context, docs, conversationID)
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
	var toolsUsed []ToolUsed
	eventCount := 0

	log.Debug("processing stream events")
	for event := range streamCh {
		eventCount++
		switch event.Type {
		case OrchestratorEventToolStart:
			log.WithField("tool", event.ToolName).Debug("tool started")
			sendSSE(w, flusher, StreamEvent{
				Type:        "tool_start",
				ToolName:    event.ToolName,
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
				Description: event.Description,
				ToolSuccess: event.Success,
			})
			toolsUsed = append(toolsUsed, ToolUsed{
				Name:        event.ToolName,
				Description: event.Description,
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
		}
	}

	log.WithFields(logrus.Fields{
		"event_count":    eventCount,
		"content_length": fullContent.Len(),
		"tools_used":     len(toolsUsed),
	}).Debug("stream processing completed")

	// Send sources if available
	if len(sources) > 0 {
		log.WithField("source_count", len(sources)).Debug("sending sources")
		sendSSE(w, flusher, StreamEvent{
			Type:    "sources",
			Sources: sources,
		})
	}

	// Send done event
	log.Debug("sending done event")
	sendSSE(w, flusher, StreamEvent{
		Type: "done",
	})

	// Store messages asynchronously
	go func() {
		result := &OrchestratorResult{
			Content:   fullContent.String(),
			ToolsUsed: toolsUsed,
			Usage:     &chat.UsageStats{},
		}
		if err := h.conversations.StoreMessages(context.Background(), conversationID, req.Message, result); err != nil {
			log.WithError(err).Error("failed to store messages")
		} else {
			log.Debug("messages stored successfully")
		}
	}()

	RecordChatRequest("success", true, 0)
	RecordToolIterations(len(toolsUsed))
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
