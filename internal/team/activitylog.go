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
	activityLogEntryActionConfirmDeleteKey                                           = "CONFIRM_DELETE_KEY"
	activityLogEntryActionSetMemberRole                                              = "SET_MEMBER_ROLE"
	activityLogEntryActionUpdateEnvironment                                          = "UPDATE_ENVIRONMENT"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeTeam, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return TeamCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created team"),
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.TransformData(entry, func(data *TeamUpdatedActivityLogData) *TeamUpdatedActivityLogData {
				if len(data.UpdatedFields) == 0 {
					return &TeamUpdatedActivityLogData{}
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
			data, err := activitylog.TransformData(entry, func(data *TeamMemberAddedActivityLogData) *TeamMemberAddedActivityLogData {
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
			data, err := activitylog.TransformData(entry, func(data *TeamMemberRemovedActivityLogData) *TeamMemberRemovedActivityLogData {
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
			data, err := activitylog.TransformData(entry, func(data *TeamMemberSetRoleActivityLogData) *TeamMemberSetRoleActivityLogData {
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
			data, err := activitylog.TransformData(entry, func(data *TeamEnvironmentUpdatedActivityLogData) *TeamEnvironmentUpdatedActivityLogData {
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
	Data *TeamUpdatedActivityLogData `json:"data"`
}

type TeamUpdatedActivityLogData struct {
	UpdatedFields []*TeamUpdatedActivityLogDataUpdatedField `json:"updatedFields"`
}

type TeamUpdatedActivityLogDataUpdatedField struct {
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
	Data *TeamMemberAddedActivityLogData `json:"data"`
}

type TeamMemberAddedActivityLogData struct {
	Role      TeamMemberRole `json:"role"`
	UserUUID  uuid.UUID      `json:"userID"`
	UserEmail string         `json:"userEmail"`
}

func (t TeamMemberAddedActivityLogData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamMemberRemovedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamMemberRemovedActivityLogData `json:"data"`
}

type TeamMemberRemovedActivityLogData struct {
	UserUUID  uuid.UUID `json:"userID"`
	UserEmail string    `json:"userEmail"`
}

func (t TeamMemberRemovedActivityLogData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamMemberSetRoleActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamMemberSetRoleActivityLogData `json:"data"`
}

type TeamMemberSetRoleActivityLogData struct {
	Role      TeamMemberRole `json:"role"`
	UserUUID  uuid.UUID      `json:"userID"`
	UserEmail string         `json:"userEmail"`
}

func (t TeamMemberSetRoleActivityLogData) UserID() ident.Ident {
	return user.NewIdent(t.UserUUID)
}

type TeamEnvironmentUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *TeamEnvironmentUpdatedActivityLogData `json:"data"`
}

type TeamEnvironmentUpdatedActivityLogData struct {
	UpdatedFields []*TeamEnvironmentUpdatedActivityLogDataUpdatedField `json:"updatedFields"`
}

type TeamEnvironmentUpdatedActivityLogDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}
