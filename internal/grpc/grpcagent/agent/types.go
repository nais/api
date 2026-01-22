package agent

import (
	"time"

	"github.com/google/uuid"
)

// ChatContext contains the user's current UI context.
type ChatContext struct {
	Path string `json:"path,omitempty"`
	Team string `json:"team,omitempty"`
	App  string `json:"app,omitempty"`
	Env  string `json:"env,omitempty"`
}

// ToolUsed describes a tool that was invoked.
type ToolUsed struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Source describes a documentation source used in the response.
type Source struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ConversationSummary represents a conversation in list view.
type ConversationSummary struct {
	ID        uuid.UUID `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Conversation represents a full conversation with messages.
type Conversation struct {
	ID        uuid.UUID             `json:"id"`
	Title     string                `json:"title"`
	Messages  []ConversationMessage `json:"messages"`
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
}

// ConversationMessage represents a message in a conversation.
type ConversationMessage struct {
	ID        uuid.UUID  `json:"id"`
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolsUsed []ToolUsed `json:"tools_used,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
