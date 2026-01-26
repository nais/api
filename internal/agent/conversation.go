package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/agent/agentsql"
	"github.com/nais/api/internal/agent/chat"
)

const maxConversationsPerUser = 10

// ConversationStore handles database operations for conversations.
type ConversationStore struct {
	querier agentsql.Querier
	pool    *pgxpool.Pool
}

// NewConversationStore creates a new conversation store.
func NewConversationStore(pool *pgxpool.Pool) *ConversationStore {
	return &ConversationStore{
		querier: agentsql.New(pool),
		pool:    pool,
	}
}

// GetOrCreateConversation retrieves an existing conversation or creates a new one.
func (s *ConversationStore) GetOrCreateConversation(ctx context.Context, userID uuid.UUID, conversationID string, firstMessage string) (uuid.UUID, error) {
	if conversationID != "" {
		id, err := uuid.Parse(conversationID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid conversation ID: %w", err)
		}

		// Verify the conversation exists and belongs to the user
		exists, err := s.querier.ConversationExists(ctx, agentsql.ConversationExistsParams{
			ID:     id,
			UserID: userID,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to check conversation: %w", err)
		}
		if !exists {
			return uuid.Nil, fmt.Errorf("conversation not found")
		}

		// Update the conversation's updated_at timestamp
		if err := s.querier.TouchConversation(ctx, id); err != nil {
			return uuid.Nil, fmt.Errorf("failed to update conversation: %w", err)
		}

		return id, nil
	}

	// Create a new conversation
	return s.createConversation(ctx, userID, firstMessage)
}

func (s *ConversationStore) createConversation(ctx context.Context, userID uuid.UUID, firstMessage string) (uuid.UUID, error) {
	// Generate title from first message (truncate if too long)
	title := generateTitle(firstMessage)

	// Start a transaction
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Use querier with transaction
	qtx := agentsql.New(tx)

	// Enforce max conversations per user - delete oldest if at limit
	if err := s.enforceConversationLimit(ctx, qtx, userID); err != nil {
		return uuid.Nil, fmt.Errorf("failed to enforce conversation limit: %w", err)
	}

	// Create the new conversation
	id, err := qtx.CreateConversation(ctx, agentsql.CreateConversationParams{
		UserID: userID,
		Title:  title,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return id, nil
}

func (s *ConversationStore) enforceConversationLimit(ctx context.Context, querier agentsql.Querier, userID uuid.UUID) error {
	// Count current conversations
	count, err := querier.CountConversations(ctx, userID)
	if err != nil {
		return err
	}

	// If at or above limit, delete the oldest
	if count >= maxConversationsPerUser {
		deleteCount := int32(count - maxConversationsPerUser + 1)
		if err := querier.DeleteOldestConversations(ctx, agentsql.DeleteOldestConversationsParams{
			UserID:     userID,
			LimitCount: deleteCount,
		}); err != nil {
			return fmt.Errorf("failed to delete old conversations: %w", err)
		}
	}

	return nil
}

// StoreMessages stores the user message and assistant response in the conversation.
func (s *ConversationStore) StoreMessages(ctx context.Context, conversationID uuid.UUID, userMessage string, result *OrchestratorResult) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := agentsql.New(tx)

	// Store user message
	if err := qtx.InsertMessage(ctx, agentsql.InsertMessageParams{
		ConversationID: conversationID,
		Role:           "user",
		Content:        userMessage,
		ToolCalls:      nil,
	}); err != nil {
		return fmt.Errorf("failed to store user message: %w", err)
	}

	// Store assistant message
	var toolCallsJSON []byte
	if len(result.ToolsUsed) > 0 {
		toolCallsJSON, err = json.Marshal(result.ToolsUsed)
		if err != nil {
			return fmt.Errorf("failed to marshal tool calls: %w", err)
		}
	}

	if err := qtx.InsertMessage(ctx, agentsql.InsertMessageParams{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        result.Content,
		ToolCalls:      toolCallsJSON,
	}); err != nil {
		return fmt.Errorf("failed to store assistant message: %w", err)
	}

	// Update conversation's updated_at
	if err := qtx.TouchConversation(ctx, conversationID); err != nil {
		return fmt.Errorf("failed to update conversation timestamp: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// ListConversations returns all conversations for a user, ordered by most recent.
func (s *ConversationStore) ListConversations(ctx context.Context, userID uuid.UUID) ([]ConversationSummary, error) {
	rows, err := s.querier.ListConversations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query conversations: %w", err)
	}

	conversations := make([]ConversationSummary, 0, len(rows))
	for _, row := range rows {
		conversations = append(conversations, ConversationSummary{
			ID:        row.ID,
			Title:     row.Title,
			UpdatedAt: row.UpdatedAt.Time,
		})
	}

	return conversations, nil
}

// GetConversation retrieves a full conversation with all messages.
func (s *ConversationStore) GetConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) (*Conversation, error) {
	// Get conversation metadata
	row, err := s.querier.GetConversation(ctx, agentsql.GetConversationParams{
		ID:     conversationID,
		UserID: userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("conversation not found")
		}
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	conv := &Conversation{
		ID:        row.ID,
		Title:     row.Title,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}

	// Get messages
	messages, err := s.querier.GetConversationMessages(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}

	conv.Messages = make([]ConversationMessage, 0, len(messages))
	for _, msg := range messages {
		cm := ConversationMessage{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Time,
		}

		if msg.ToolCalls != nil {
			if err := json.Unmarshal(msg.ToolCalls, &cm.ToolsUsed); err != nil {
				// Log but don't fail - tools_used is optional
				cm.ToolsUsed = nil
			}
		}

		conv.Messages = append(conv.Messages, cm)
	}

	return conv, nil
}

// DeleteConversation deletes a conversation and all its messages.
func (s *ConversationStore) DeleteConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) error {
	rowsAffected, err := s.querier.DeleteConversation(ctx, agentsql.DeleteConversationParams{
		ID:     conversationID,
		UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("conversation not found")
	}

	return nil
}

// GetConversationHistory retrieves the message history for a conversation, formatted for the LLM.
func (s *ConversationStore) GetConversationHistory(ctx context.Context, conversationID uuid.UUID) ([]chat.Message, error) {
	rows, err := s.querier.GetConversationHistory(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}

	messages := make([]chat.Message, 0, len(rows))
	for _, row := range rows {
		msg := chat.Message{
			Role:    chat.Role(row.Role),
			Content: row.Content,
		}

		if row.ToolCallID != nil {
			msg.ToolCallID = *row.ToolCallID
		}

		if row.ToolCalls != nil {
			var toolCalls []chat.ToolCall
			if err := json.Unmarshal(row.ToolCalls, &toolCalls); err == nil {
				msg.ToolCalls = toolCalls
			}
		}

		messages = append(messages, msg)
	}

	return messages, nil
}

// generateTitle creates a title from the first message.
func generateTitle(message string) string {
	// Trim whitespace and limit length
	title := strings.TrimSpace(message)

	// Take first line only
	if idx := strings.Index(title, "\n"); idx > 0 {
		title = title[:idx]
	}

	// Truncate to reasonable length
	maxLen := 100
	if len(title) > maxLen {
		title = title[:maxLen-3] + "..."
	}

	if title == "" {
		title = "New conversation"
	}

	return title
}
