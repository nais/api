package usersync

import (
	"time"

	"github.com/nais/api/internal/graph/pagination"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
)

type UserSyncLogEntry interface {
	model.Node
	IsUserSyncLogEntry()
}

type (
	UserSyncLogEntryConnection = pagination.Connection[UserSyncLogEntry]
	UserSyncLogEntryEdge       = pagination.Edge[UserSyncLogEntry]
)

type GenericUserSyncLogEntry struct {
	ID        ident.Ident `json:"id"`
	CreatedAt time.Time   `json:"createdAt"`
	Message   string      `json:"message"`
	UserName  string      `json:"userName"`
	UserEmail string      `json:"userEmail"`
}

func (GenericUserSyncLogEntry) IsUserSyncLogEntry() {}
func (GenericUserSyncLogEntry) IsNode()             {}

type UserCreatedUserSyncLogEntry struct {
	GenericUserSyncLogEntry
}

type UserDeletedUserSyncLogEntry struct {
	GenericUserSyncLogEntry
}

type UserUpdatedUserSyncLogEntry struct {
	GenericUserSyncLogEntry
	OldUserName  string `json:"oldUserName"`
	OldUserEmail string `json:"oldUserEmail"`
}

type AssignedRoleUserSyncLogEntry struct {
	GenericUserSyncLogEntry
	Role string `json:"role"`
}

type RevokedRoleUserSyncLogEntry struct {
	GenericUserSyncLogEntry
	Role string `json:"role"`
}
