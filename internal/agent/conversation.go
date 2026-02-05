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
		Blocks:         nil,
	}); err != nil {
		return fmt.Errorf("failed to store user message: %w", err)
	}

	// Store assistant message with blocks
	var blocksJSON []byte
	if len(result.Blocks) > 0 {
		blocksJSON, err = json.Marshal(result.Blocks)
		if err != nil {
			return fmt.Errorf("failed to marshal blocks: %w", err)
		}
	}

	// Assistant messages store content in blocks; content column left empty
	if err := qtx.InsertMessage(ctx, agentsql.InsertMessageParams{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        "",
		Blocks:         blocksJSON,
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

		// Parse blocks if present
		if msg.Blocks != nil {
			if err := json.Unmarshal(msg.Blocks, &cm.Blocks); err != nil {
				// Log but don't fail - blocks is optional
				cm.Blocks = nil
			}
			// For assistant messages, derive content from blocks
			if msg.Role == "assistant" && cm.Content == "" {
				cm.Content = extractTextContentFromBlocks(cm.Blocks)
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
// This reconstructs the full conversation including tool call responses from stored blocks.
func (s *ConversationStore) GetConversationHistory(ctx context.Context, conversationID uuid.UUID) ([]chat.Message, error) {
	rows, err := s.querier.GetConversationHistory(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}

	messages := make([]chat.Message, 0, len(rows)*2) // Pre-allocate for potential tool messages
	for _, row := range rows {
		if row.Role == "user" {
			// User messages are stored directly
			messages = append(messages, chat.Message{
				Role:    chat.RoleUser,
				Content: row.Content,
			})
			continue
		}

		if row.Role == "assistant" && row.Blocks != nil {
			// Assistant messages need to be reconstructed from blocks.
			// The blocks contain tool calls and their results in order.
			var blocks []ContentBlock
			if err := json.Unmarshal(row.Blocks, &blocks); err != nil {
				// If we can't parse blocks, just add a simple assistant message
				messages = append(messages, chat.Message{
					Role:    chat.RoleAssistant,
					Content: row.Content,
				})
				continue
			}

			// Reconstruct the conversation from blocks:
			// 1. Collect tool calls made by the assistant
			// 2. For each tool call, add the tool response
			// 3. Add the final text response
			toolCalls := extractToolCallsFromBlocks(blocks)
			textContent := extractTextContentFromBlocks(blocks)

			if len(toolCalls) > 0 {
				// Add assistant message with tool calls
				messages = append(messages, chat.Message{
					Role:      chat.RoleAssistant,
					Content:   "", // Content before tool calls is typically empty or minimal
					ToolCalls: toolCalls,
				})

				// Add tool response messages in order
				for _, block := range blocks {
					if block.Type == ContentBlockTypeToolUse && block.ToolResult != "" {
						messages = append(messages, chat.Message{
							Role:       chat.RoleTool,
							Content:    block.ToolResult,
							ToolCallID: block.ToolCallID,
						})
					}
				}

				// Add final assistant response if there's text content
				if textContent != "" {
					messages = append(messages, chat.Message{
						Role:    chat.RoleAssistant,
						Content: textContent,
					})
				}
			} else {
				// No tool calls, just a simple assistant message
				messages = append(messages, chat.Message{
					Role:    chat.RoleAssistant,
					Content: textContent,
				})
			}
			continue
		}

		// Fallback for other roles or missing blocks
		messages = append(messages, chat.Message{
			Role:    chat.Role(row.Role),
			Content: row.Content,
		})
	}

	return messages, nil
}

// extractToolCallsFromBlocks extracts tool call information from blocks for LLM history.
func extractToolCallsFromBlocks(blocks []ContentBlock) []chat.ToolCall {
	var toolCalls []chat.ToolCall
	for _, block := range blocks {
		if block.Type == ContentBlockTypeToolUse {
			toolCalls = append(toolCalls, chat.ToolCall{
				ID:   block.ToolCallID,
				Name: block.ToolName,
			})
		}
	}
	return toolCalls
}

// extractTextContentFromBlocks extracts all text content from blocks.
func extractTextContentFromBlocks(blocks []ContentBlock) string {
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
