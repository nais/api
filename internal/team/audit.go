package team

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/user"
)

const (
	auditResourceTypeTeam        audit.AuditResourceType = "TEAM"
	auditActionCreateDeleteKey   audit.AuditAction       = "CREATE_DELETE_KEY"
	auditActionConfirmDeleteKey                          = "CONFIRM_DELETE_KEY"
	auditActionAddMember                                 = "ADD_MEMBER"
	auditActionRemoveMember                              = "REMOVE_MEMBER"
	auditActionSetMemberRole                             = "SET_MEMBER_ROLE"
	auditActionUpdateEnvironment                         = "UPDATE_ENVIRONMENT"
)

func init() {
	audit.RegisterTransformer(auditResourceTypeTeam, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case audit.AuditActionCreated:
			return TeamCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created team"),
			}, nil
		case audit.AuditActionUpdated:
			data, err := audit.TransformData(entry, func(data *TeamUpdatedAuditEntryData) *TeamUpdatedAuditEntryData {
				if len(data.UpdatedFields) == 0 {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated team"),
				Data:              data,
			}, nil
		case auditActionCreateDeleteKey:
			return TeamCreateDeleteKeyAuditEntry{
				GenericAuditEntry: entry.WithMessage("Create delete key"),
			}, nil
		case auditActionConfirmDeleteKey:
			return TeamConfirmDeleteKeyAuditEntry{
				GenericAuditEntry: entry.WithMessage("Confirm delete key"),
			}, nil
		case auditActionAddMember:
			data, err := audit.TransformData(entry, func(data *TeamMemberAddedAuditEntryData) *TeamMemberAddedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberAddedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Add member"),
				Data:              data,
			}, nil
		case auditActionRemoveMember:
			data, err := audit.TransformData(entry, func(data *TeamMemberRemovedAuditEntryData) *TeamMemberRemovedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberRemovedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Remove member"),
				Data:              data,
			}, nil
		case auditActionSetMemberRole:
			data, err := audit.TransformData(entry, func(data *TeamMemberSetRoleAuditEntryData) *TeamMemberSetRoleAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberSetRoleAuditEntry{
				GenericAuditEntry: entry.WithMessage("Set member role"),
				Data:              data,
			}, nil
		case auditActionUpdateEnvironment:
			data, err := audit.TransformData(entry, func(data *TeamEnvironmentUpdatedAuditEntryData) *TeamEnvironmentUpdatedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamEnvironmentUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Update environment"),
				Data:              data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported team audit entry action: %q", entry.Action)
		}
	})
}

type TeamCreatedAuditEntry struct {
	audit.GenericAuditEntry
}

type TeamUpdatedAuditEntry struct {
	audit.GenericAuditEntry
	Data *TeamUpdatedAuditEntryData `json:"data"`
}

type TeamUpdatedAuditEntryData struct {
	UpdatedFields []*TeamUpdatedAuditEntryDataUpdatedField `json:"updatedFields"`
}

type TeamUpdatedAuditEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}

type TeamConfirmDeleteKeyAuditEntry struct {
	audit.GenericAuditEntry
}

type TeamCreateDeleteKeyAuditEntry struct {
	audit.GenericAuditEntry
}

type TeamMemberAddedAuditEntry struct {
	audit.GenericAuditEntry
	Data *TeamMemberAddedAuditEntryData `json:"data"`
}

type TeamMemberAddedAuditEntryData struct {
	Role      TeamMemberRole `json:"role"`
	UserUUID  uuid.UUID      `json:"userID"`
	UserEmail string         `json:"userEmail"`
}

func (t TeamMemberAddedAuditEntryData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamMemberRemovedAuditEntry struct {
	audit.GenericAuditEntry
	Data *TeamMemberRemovedAuditEntryData `json:"data"`
}

type TeamMemberRemovedAuditEntryData struct {
	UserUUID  uuid.UUID `json:"userID"`
	UserEmail string    `json:"userEmail"`
}

func (t TeamMemberRemovedAuditEntryData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamMemberSetRoleAuditEntry struct {
	audit.GenericAuditEntry
	Data *TeamMemberSetRoleAuditEntryData `json:"data"`
}

type TeamMemberSetRoleAuditEntryData struct {
	Role      TeamMemberRole `json:"role"`
	UserUUID  uuid.UUID      `json:"userID"`
	UserEmail string         `json:"userEmail"`
}

func (t TeamMemberSetRoleAuditEntryData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamEnvironmentUpdatedAuditEntry struct {
	audit.GenericAuditEntry
	Data *TeamEnvironmentUpdatedAuditEntryData `json:"data"`
}

type TeamEnvironmentUpdatedAuditEntryData struct {
	UpdatedFields []*TeamEnvironmentUpdatedAuditEntryDataUpdatedField `json:"updatedFields"`
}

type TeamEnvironmentUpdatedAuditEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}
