package team

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/user"
)

const (
	auditResourceTypeTeam        auditv1.AuditResourceType = "TEAM"
	auditActionCreateDeleteKey   auditv1.AuditAction       = "CREATE_DELETE_KEY"
	auditActionConfirmDeleteKey                            = "CONFIRM_DELETE_KEY"
	auditActionAddMember                                   = "ADD_MEMBER"
	auditActionRemoveMember                                = "REMOVE_MEMBER"
	auditActionSetMemberRole                               = "SET_MEMBER_ROLE"
	auditActionUpdateEnvironment                           = "UPDATE_ENVIRONMENT"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeTeam, func(entry auditv1.GenericAuditEntry) (auditv1.AuditEntry, error) {
		switch entry.Action {
		case auditv1.AuditActionCreated:
			return TeamCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created team"),
			}, nil
		case auditv1.AuditActionUpdated:
			data, err := auditv1.TransformData(entry, func(data *TeamUpdatedAuditEntryData) *TeamUpdatedAuditEntryData {
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
			data, err := auditv1.TransformData(entry, func(data *TeamMemberAddedAuditEntryData) *TeamMemberAddedAuditEntryData {
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
			data, err := auditv1.TransformData(entry, func(data *TeamMemberRemovedAuditEntryData) *TeamMemberRemovedAuditEntryData {
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
			data, err := auditv1.TransformData(entry, func(data *TeamMemberSetRoleAuditEntryData) *TeamMemberSetRoleAuditEntryData {
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
			data, err := auditv1.TransformData(entry, func(data *TeamEnvironmentUpdatedAuditEntryData) *TeamEnvironmentUpdatedAuditEntryData {
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
	auditv1.GenericAuditEntry
}

type TeamUpdatedAuditEntry struct {
	auditv1.GenericAuditEntry
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
	auditv1.GenericAuditEntry
}

type TeamCreateDeleteKeyAuditEntry struct {
	auditv1.GenericAuditEntry
}

type TeamMemberAddedAuditEntry struct {
	auditv1.GenericAuditEntry
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
	auditv1.GenericAuditEntry
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
	auditv1.GenericAuditEntry
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
	auditv1.GenericAuditEntry
	Data *TeamEnvironmentUpdatedAuditEntryData `json:"data"`
}

type TeamEnvironmentUpdatedAuditEntryData struct {
	EnvironmentName string                                              `json:"environmentName"`
	UpdatedFields   []*TeamEnvironmentUpdatedAuditEntryDataUpdatedField `json:"updatedFields"`
}

type TeamEnvironmentUpdatedAuditEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}
