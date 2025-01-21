package usersync

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/usersync/usersyncsql"
	"k8s.io/utils/ptr"
)

type UserSyncLogEntry interface {
	model.Node
	GetUUID() uuid.UUID
}

type (
	UserSyncLogEntryConnection = pagination.Connection[UserSyncLogEntry]
	UserSyncLogEntryEdge       = pagination.Edge[UserSyncLogEntry]
)

type userSyncLogEntry struct {
	CreatedAt time.Time   `json:"createdAt"`
	Message   string      `json:"message"`
	UserID    ident.Ident `json:"userID"`
	UserName  string      `json:"userName"`
	UserEmail string      `json:"userEmail"`
	UUID      uuid.UUID   `json:"-"`
}

func (userSyncLogEntry) IsNode() {}
func (l *userSyncLogEntry) GetUUID() uuid.UUID {
	return l.UUID
}

func (l userSyncLogEntry) ID() ident.Ident {
	return newIdent(l.UUID)
}

func (l userSyncLogEntry) WithMessage(message string) userSyncLogEntry {
	l.Message = message
	return l
}

type UserCreatedUserSyncLogEntry struct {
	userSyncLogEntry
}

type UserDeletedUserSyncLogEntry struct {
	userSyncLogEntry
}

type UserUpdatedUserSyncLogEntry struct {
	userSyncLogEntry
	OldUserName  string `json:"oldUserName"`
	OldUserEmail string `json:"oldUserEmail"`
}

type RoleAssignedUserSyncLogEntry struct {
	userSyncLogEntry
	RoleName string `json:"roleName"`
}

type RoleRevokedUserSyncLogEntry struct {
	userSyncLogEntry
	RoleName string `json:"roleName"`
}

func toGraphUserSyncLogEntry(row *usersyncsql.UsersyncLogEntry) (UserSyncLogEntry, error) {
	entry := userSyncLogEntry{
		UUID:      row.ID,
		CreatedAt: row.CreatedAt.Time,
		UserID:    newIdent(row.UserID),
		UserName:  row.UserName,
		UserEmail: row.UserEmail,
	}

	switch row.Action {
	case usersyncsql.UsersyncLogEntryActionCreateUser:
		return &UserCreatedUserSyncLogEntry{
			userSyncLogEntry: entry.WithMessage("Created user"),
		}, nil
	case usersyncsql.UsersyncLogEntryActionUpdateUser:
		return &UserUpdatedUserSyncLogEntry{
			userSyncLogEntry: entry.WithMessage("Updated user"),
			OldUserName:      ptr.Deref(row.OldUserName, "unknown"),
			OldUserEmail:     ptr.Deref(row.OldUserEmail, "unknown"),
		}, nil
	case usersyncsql.UsersyncLogEntryActionDeleteUser:
		return &UserDeletedUserSyncLogEntry{
			userSyncLogEntry: entry.WithMessage("Deleted user"),
		}, nil
	case usersyncsql.UsersyncLogEntryActionAssignRole:
		return &RoleAssignedUserSyncLogEntry{
			userSyncLogEntry: entry.WithMessage("Assigned role"),
			RoleName:         ptr.Deref(row.RoleName, "unknown"),
		}, nil
	case usersyncsql.UsersyncLogEntryActionRevokeRole:
		return &RoleRevokedUserSyncLogEntry{
			userSyncLogEntry: entry.WithMessage("Revoked role"),
			RoleName:         ptr.Deref(row.RoleName, "unknown"),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported user sync log entry action: %q", row.Action)
	}
}
