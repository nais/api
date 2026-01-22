// Package agent provides the AI chat service for the Nais platform.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/agent/rag"
	"github.com/nais/api/internal/agent/tools"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/sirupsen/logrus"
)

const maxRAGResults = 5

// Handler implements the HTTP handler for the agent chat service.
type Handler struct {
	chatClient chat.StreamingClient
	ragClient  rag.DocumentSearcher
	registry   *tools.Registry
	log        logrus.FieldLogger
}

// Config holds configuration for the agent handler.
type Config struct {
	ChatClient     chat.StreamingClient
	RAGClient      rag.DocumentSearcher
	GraphQLHandler *handler.Server
	TenantName     string
	Log            logrus.FieldLogger
}

// NewHandler creates a new agent HTTP handler.
func NewHandler(cfg Config) (*Handler, error) {
	if cfg.GraphQLHandler == nil {
		return nil, fmt.Errorf("GraphQLHandler is required")
	}

	internalClient := NewInternalClient(
		cfg.GraphQLHandler,
		cfg.Log.WithField("component", "internal_client"),
	)

	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
	consoleBaseURL, urlPatterns := buildConsoleURLs(cfg.TenantName)

	registry := tools.NewRegistry(tools.RegistryConfig{
		Client:             internalClient,
		Schema:             schema,
		ConsoleBaseURL:     consoleBaseURL,
		ConsoleURLPatterns: urlPatterns,
	})

	return &Handler{
		chatClient: cfg.ChatClient,
		ragClient:  cfg.RAGClient,
		registry:   registry,
		log:        cfg.Log,
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

// chatSetup holds the prepared state for a conversation turn, assembled by
// prepareConversation before being handed off to Chat or ChatStream.
type chatSetup struct {
	Request        ChatRequest
	ConversationID uuid.UUID
	History        []chat.Message
	Docs           []rag.Document
	Sources        []Source
}

// prepareConversation handles the common setup for chat requests:
// parses the request, authenticates the user, loads conversation history,
// and performs a RAG search for relevant documentation.
func (h *Handler) prepareConversation(w http.ResponseWriter, r *http.Request, log logrus.FieldLogger) (*chatSetup, error) {
	ctx := r.Context()

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.WithError(err).Debug("failed to decode request body")
		writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return nil, err
	}

	if strings.TrimSpace(req.Message) == "" {
		writeJSONError(w, http.StatusBadRequest, "message is required")
		return nil, fmt.Errorf("empty message")
	}

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		log.WithError(err).Debug("failed to get user from context")
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return nil, err
	}

	conversationID, err := GetOrCreateConversation(ctx, userID, req.ConversationID, req.Message)
	if err != nil {
		log.WithError(err).Error("failed to get or create conversation")
		writeJSONError(w, http.StatusInternalServerError, "failed to process conversation")
		return nil, err
	}

	history, err := GetConversationHistory(ctx, conversationID)
	if err != nil {
		log.WithError(err).Warn("failed to load conversation history, continuing without")
		history = nil
	}

	docs, sources, err := h.searchDocumentation(ctx, req.Message)
	if err != nil {
		log.WithError(err).Warn("RAG search failed, continuing without docs")
	}

	return &chatSetup{
		Request:        req,
		ConversationID: conversationID,
		History:        history,
		Docs:           docs,
		Sources:        sources,
	}, nil
}

// Chat handles non-streaming chat requests.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.WithField("method", "Chat")

	setup, err := h.prepareConversation(w, r, log)
	if err != nil {
		return
	}

	log = log.WithField("conversation_id", setup.ConversationID)

	orchestrator := NewOrchestrator(h.chatClient, h.registry, log)

	result, err := orchestrator.Run(ctx, setup.Request.Message, setup.Request.Context, setup.Docs, setup.History)
	if err != nil {
		log.WithError(err).Error("orchestrator failed")
		writeJSONError(w, http.StatusInternalServerError, "failed to process chat request")
		return
	}

	if err := StoreMessages(ctx, setup.ConversationID, setup.Request.Message, result, setup.Sources); err != nil {
		log.WithError(err).Error("failed to store messages")
	}

	textContent := extractTextContentFromBlocks(result.Blocks)

	writeJSON(w, http.StatusOK, ChatResponse{
		ConversationID: setup.ConversationID.String(),
		MessageID:      uuid.New().String(),
		Content:        textContent,
		Blocks:         clientVisibleBlocks(result.Blocks),
		Sources:        setup.Sources,
		Usage:          result.Usage,
	})
}

// ChatStream handles streaming chat requests using Server-Sent Events.
func (h *Handler) ChatStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := h.log.WithField("method", "ChatStream")

	setup, err := h.prepareConversation(w, r, log)
	if err != nil {
		return
	}

	log = log.WithField("conversation_id", setup.ConversationID)

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

	messageID := uuid.New().String()
	sendSSE(w, flusher, StreamEvent{
		Type:           "metadata",
		ConversationID: setup.ConversationID.String(),
		MessageID:      messageID,
	})

	orchestrator := NewOrchestrator(h.chatClient, h.registry, log)
	streamCh := orchestrator.RunStream(ctx, setup.Request.Message, setup.Request.Context, setup.Docs, setup.History)

	for event := range streamCh {
		switch event.Type {
		case OrchestratorEventError:
			log.WithError(event.Error).Error("stream error")
			sendSSE(w, flusher, StreamEvent{
				Type:         "error",
				ErrorCode:    "stream_error",
				ErrorMessage: event.Error.Error(),
			})
			return

		case OrchestratorEventDone:
			result := event.Result

			if result.Usage != nil {
				sendSSE(w, flusher, StreamEvent{
					Type:  "usage",
					Usage: result.Usage,
				})
			}

			if len(setup.Sources) > 0 {
				sendSSE(w, flusher, StreamEvent{
					Type:    "sources",
					Sources: setup.Sources,
				})
			}

			sendSSE(w, flusher, StreamEvent{Type: "done"})

			// Store messages asynchronously after the SSE stream is flushed.
			// context.WithoutCancel preserves all context values (loaders, db pool, etc.)
			// while detaching from the request's cancellation signal.
			storeCtx := context.WithoutCancel(ctx)
			go func(r *OrchestratorResult) {
				if err := StoreMessages(storeCtx, setup.ConversationID, setup.Request.Message, r, setup.Sources); err != nil {
					log.WithError(err).Error("failed to store messages")
				}
			}(result)

			return

		default:
			sendSSE(w, flusher, event.ToStreamEvent())
		}
	}

	// If the channel closed without a Done event (unexpected), send a done anyway.
	sendSSE(w, flusher, StreamEvent{Type: "done"})
}

// ListConversations returns all conversations for the authenticated user.
func (h *Handler) ListConversations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID, err := getUserIDFromContext(ctx)
	if err != nil {
		writeJSONError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	conversations, err := ListConversations(ctx, userID)
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

	conv, err := GetConversation(ctx, userID, conversationID)
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

	if err := DeleteConversation(ctx, userID, conversationID); err != nil {
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

	// Deduplicate sources by URL (multiple chunks from the same page → one source entry).
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

// getUserIDFromContext extracts the user ID from the HTTP request context.
func getUserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	actor := authz.ActorFromContext(ctx)
	if actor == nil || actor.User == nil {
		return uuid.Nil, fmt.Errorf("no user in context")
	}
	return actor.User.GetID(), nil
}

// buildConsoleURLs builds the console base URL and URL patterns for a tenant.
func buildConsoleURLs(tenantName string) (string, map[string]string) {
	baseURL := fmt.Sprintf("https://console.%s.cloud.nais.io", tenantName)

	patterns := map[string]string{
		"team":        baseURL + "/team/{team}",
		"app":         baseURL + "/team/{team}/{env}/app/{app}",
		"job":         baseURL + "/team/{team}/{env}/job/{job}",
		"deployment":  baseURL + "/team/{team}/deployments",
		"cost":        baseURL + "/team/{team}/cost",
		"utilization": baseURL + "/team/{team}/utilization",
		"secrets":     baseURL + "/team/{team}/{env}/secret/{secret}",
		"postgres":    baseURL + "/team/{team}/{env}/postgres/{instance}",
		"bucket":      baseURL + "/team/{team}/{env}/bucket/{bucket}",
		"redis":       baseURL + "/team/{team}/{env}/redis/{instance}",
		"opensearch":  baseURL + "/team/{team}/{env}/opensearch/{instance}",
		"kafka":       baseURL + "/team/{team}/{env}/kafka/{topic}",
	}

	return baseURL, patterns
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
