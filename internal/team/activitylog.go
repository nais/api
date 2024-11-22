package team

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/user"
)

const (
	activityLogEntryResourceTypeTeam        activitylog.ActivityLogEntryResourceType = "TEAM"
	activityLogEntryActionCreateDeleteKey   activitylog.ActivityLogEntryAction       = "CREATE_DELETE_KEY"
	activityLogEntryActionConfirmDeleteKey  activitylog.ActivityLogEntryAction       = "CONFIRM_DELETE_KEY"
	activityLogEntryActionSetMemberRole     activitylog.ActivityLogEntryAction       = "SET_MEMBER_ROLE"
	activityLogEntryActionUpdateEnvironment activitylog.ActivityLogEntryAction       = "UPDATE_ENVIRONMENT"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeTeam, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return TeamCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created team"),
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.TransformData(entry, func(data *TeamUpdatedActivityLogEntryData) *TeamUpdatedActivityLogEntryData {
				if len(data.UpdatedFields) == 0 {
					return &TeamUpdatedActivityLogEntryData{}
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated team"),
				Data:                    data,
			}, nil
		case activityLogEntryActionCreateDeleteKey:
			return TeamCreateDeleteKeyActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Create delete key"),
			}, nil
		case activityLogEntryActionConfirmDeleteKey:
			return TeamConfirmDeleteKeyActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Confirm delete key"),
			}, nil
		case activitylog.ActivityLogEntryActionAdded:
			data, err := activitylog.TransformData(entry, func(data *TeamMemberAddedActivityLogEntryData) *TeamMemberAddedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberAddedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Add member"),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionRemoved:
			data, err := activitylog.TransformData(entry, func(data *TeamMemberRemovedActivityLogEntryData) *TeamMemberRemovedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberRemovedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Remove member"),
				Data:                    data,
			}, nil
		case activityLogEntryActionSetMemberRole:
			data, err := activitylog.TransformData(entry, func(data *TeamMemberSetRoleActivityLogEntryData) *TeamMemberSetRoleActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberSetRoleActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Set member role"),
				Data:                    data,
			}, nil
		case activityLogEntryActionUpdateEnvironment:
			data, err := activitylog.TransformData(entry, func(data *TeamEnvironmentUpdatedActivityLogEntryData) *TeamEnvironmentUpdatedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamEnvironmentUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Update environment"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported team activity log entry action: %q", entry.Action)
		}
	})
}

type TeamCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type TeamUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamUpdatedActivityLogEntryData `json:"data"`
}

type TeamUpdatedActivityLogEntryData struct {
	UpdatedFields []*TeamUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type TeamUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}

type TeamConfirmDeleteKeyActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type TeamCreateDeleteKeyActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type TeamMemberAddedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamMemberAddedActivityLogEntryData `json:"data"`
}

type TeamMemberAddedActivityLogEntryData struct {
	Role      TeamMemberRole `json:"role"`
	UserUUID  uuid.UUID      `json:"userID"`
	UserEmail string         `json:"userEmail"`
}

func (t TeamMemberAddedActivityLogEntryData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamMemberRemovedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamMemberRemovedActivityLogEntryData `json:"data"`
}

type TeamMemberRemovedActivityLogEntryData struct {
	UserUUID  uuid.UUID `json:"userID"`
	UserEmail string    `json:"userEmail"`
}

func (t TeamMemberRemovedActivityLogEntryData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamMemberSetRoleActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamMemberSetRoleActivityLogEntryData `json:"data"`
}

type TeamMemberSetRoleActivityLogEntryData struct {
	Role      TeamMemberRole `json:"role"`
	UserUUID  uuid.UUID      `json:"userID"`
	UserEmail string         `json:"userEmail"`
}

func (t TeamMemberSetRoleActivityLogEntryData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamEnvironmentUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamEnvironmentUpdatedActivityLogEntryData `json:"data"`
}

type TeamEnvironmentUpdatedActivityLogEntryData struct {
	UpdatedFields []*TeamEnvironmentUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type TeamEnvironmentUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}
