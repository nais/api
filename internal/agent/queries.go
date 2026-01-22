package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/agent/agentsql"
	"github.com/nais/api/internal/agent/chat"
	"github.com/nais/api/internal/database"
)

// GetOrCreateConversation retrieves an existing conversation or creates a new one.
func GetOrCreateConversation(ctx context.Context, userID uuid.UUID, conversationID string, firstMessage string) (uuid.UUID, error) {
	if conversationID != "" {
		id, err := uuid.Parse(conversationID)
		if err != nil {
			return uuid.Nil, fmt.Errorf("invalid conversation ID: %w", err)
		}

		exists, err := db(ctx).ConversationExists(ctx, agentsql.ConversationExistsParams{
			ID:     id,
			UserID: userID,
		})
		if err != nil {
			return uuid.Nil, fmt.Errorf("failed to check conversation: %w", err)
		}
		if !exists {
			return uuid.Nil, fmt.Errorf("conversation not found")
		}

		if err := db(ctx).TouchConversation(ctx, id); err != nil {
			return uuid.Nil, fmt.Errorf("failed to update conversation: %w", err)
		}

		return id, nil
	}

	return createConversation(ctx, userID, firstMessage)
}

func createConversation(ctx context.Context, userID uuid.UUID, firstMessage string) (uuid.UUID, error) {
	id, err := db(ctx).CreateConversation(ctx, agentsql.CreateConversationParams{
		UserID: userID,
		Title:  generateTitle(firstMessage),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	return id, nil
}

// StoreMessages stores the user message and assistant response in the conversation.
// sources are the RAG documents surfaced during this turn and are stored with the
// assistant message so they can be returned when the conversation is loaded later.
func StoreMessages(ctx context.Context, conversationID uuid.UUID, userMessage string, result *OrchestratorResult, sources []Source) error {
	return database.Transaction(ctx, func(ctx context.Context) error {
		if err := db(ctx).InsertMessage(ctx, agentsql.InsertMessageParams{
			ConversationID: conversationID,
			Role:           "user",
			Content:        userMessage,
			Blocks:         nil,
			Sources:        nil,
		}); err != nil {
			return fmt.Errorf("failed to store user message: %w", err)
		}

		var blocksJSON []byte
		if len(result.Blocks) > 0 {
			var err error
			blocksJSON, err = json.Marshal(result.Blocks)
			if err != nil {
				return fmt.Errorf("failed to marshal blocks: %w", err)
			}
		}

		var sourcesJSON []byte
		if len(sources) > 0 {
			var err error
			sourcesJSON, err = json.Marshal(sources)
			if err != nil {
				return fmt.Errorf("failed to marshal sources: %w", err)
			}
		}

		if err := db(ctx).InsertMessage(ctx, agentsql.InsertMessageParams{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        "",
			Blocks:         blocksJSON,
			Sources:        sourcesJSON,
		}); err != nil {
			return fmt.Errorf("failed to store assistant message: %w", err)
		}

		if err := db(ctx).TouchConversation(ctx, conversationID); err != nil {
			return fmt.Errorf("failed to update conversation timestamp: %w", err)
		}

		return nil
	})
}

// ListConversations returns all conversations for a user, ordered by most recent.
func ListConversations(ctx context.Context, userID uuid.UUID) ([]ConversationSummary, error) {
	rows, err := db(ctx).ListConversations(ctx, userID)
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
// Tool-use blocks are filtered from the returned messages since they are
// internal LLM plumbing and not meaningful to clients.
func GetConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) (*Conversation, error) {
	row, err := db(ctx).GetConversation(ctx, agentsql.GetConversationParams{
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

	messages, err := db(ctx).GetConversationMessages(ctx, conversationID)
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

		if msg.Blocks != nil {
			var allBlocks []ContentBlock
			if err := json.Unmarshal(msg.Blocks, &allBlocks); err == nil {
				// Filter out tool_use blocks — they are internal LLM history details.
				cm.Blocks = clientVisibleBlocks(allBlocks)

				if msg.Role == "assistant" && cm.Content == "" {
					cm.Content = extractTextContentFromBlocks(allBlocks)
				}
			}
		}

		if msg.Sources != nil {
			var srcs []Source
			if err := json.Unmarshal(msg.Sources, &srcs); err == nil {
				cm.Sources = srcs
			}
		}

		conv.Messages = append(conv.Messages, cm)
	}

	return conv, nil
}

// DeleteConversation deletes a conversation and all its messages.
func DeleteConversation(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) error {
	rowsAffected, err := db(ctx).DeleteConversation(ctx, agentsql.DeleteConversationParams{
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
func GetConversationHistory(ctx context.Context, conversationID uuid.UUID) ([]chat.Message, error) {
	rows, err := db(ctx).GetConversationHistory(ctx, conversationID)
	if err != nil {
		return nil, fmt.Errorf("failed to query messages: %w", err)
	}

	messages := make([]chat.Message, 0, len(rows)*2)
	for _, row := range rows {
		if row.Role == "user" {
			messages = append(messages, chat.Message{
				Role:    chat.RoleUser,
				Content: row.Content,
			})
			continue
		}

		if row.Role == "assistant" && row.Blocks != nil {
			var blocks []ContentBlock
			if err := json.Unmarshal(row.Blocks, &blocks); err != nil {
				messages = append(messages, chat.Message{
					Role:    chat.RoleAssistant,
					Content: row.Content,
				})
				continue
			}

			toolCalls := extractToolCallsFromBlocks(blocks)
			textContent := extractTextContentFromBlocks(blocks)

			if len(toolCalls) > 0 {
				messages = append(messages, chat.Message{
					Role:      chat.RoleAssistant,
					Content:   "",
					ToolCalls: toolCalls,
				})

				for _, block := range blocks {
					if block.Type == ContentBlockTypeToolUse && block.ToolResult != "" {
						messages = append(messages, chat.Message{
							Role:       chat.RoleTool,
							Content:    block.ToolResult,
							ToolCallID: block.ToolCallID,
						})
					}
				}

				if textContent != "" {
					messages = append(messages, chat.Message{
						Role:    chat.RoleAssistant,
						Content: textContent,
					})
				}
			} else {
				messages = append(messages, chat.Message{
					Role:    chat.RoleAssistant,
					Content: textContent,
				})
			}
			continue
		}

		messages = append(messages, chat.Message{
			Role:    chat.Role(row.Role),
			Content: row.Content,
		})
	}

	return messages, nil
}

// clientVisibleBlocks returns only the blocks that are meaningful to display to a client,
// filtering out internal tool_use blocks.
func clientVisibleBlocks(blocks []ContentBlock) []ContentBlock {
	result := make([]ContentBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.Type != ContentBlockTypeToolUse {
			result = append(result, b)
		}
	}
	return result
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
	title := strings.TrimSpace(message)

	if idx := strings.Index(title, "\n"); idx > 0 {
		title = title[:idx]
	}

	const maxLen = 100
	if len(title) > maxLen {
		title = title[:maxLen-3] + "..."
	}

	if title == "" {
		title = "New conversation"
	}

	return title
}
