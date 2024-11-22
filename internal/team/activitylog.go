package team

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/user"
)

const (
	activityLogResourceTypeTeam        activitylog.ActivityLogResourceType = "TEAM"
	activityLogActionCreateDeleteKey   activitylog.ActivityLogAction       = "CREATE_DELETE_KEY"
	activityLogActionConfirmDeleteKey                                      = "CONFIRM_DELETE_KEY"
	activityLogActionSetMemberRole                                         = "SET_MEMBER_ROLE"
	activityLogActionUpdateEnvironment                                     = "UPDATE_ENVIRONMENT"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeTeam, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogActionCreated:
			return TeamCreatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Created team"),
			}, nil
		case activitylog.ActivityLogActionUpdated:
			data, err := activitylog.TransformData(entry, func(data *TeamUpdatedActivityLogData) *TeamUpdatedActivityLogData {
				if len(data.UpdatedFields) == 0 {
					return &TeamUpdatedActivityLogData{}
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamUpdatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Updated team"),
				Data:                    data,
			}, nil
		case activityLogActionCreateDeleteKey:
			return TeamCreateDeleteKeyActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Create delete key"),
			}, nil
		case activityLogActionConfirmDeleteKey:
			return TeamConfirmDeleteKeyActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Confirm delete key"),
			}, nil
		case activitylog.ActivityLogActionAdded:
			data, err := activitylog.TransformData(entry, func(data *TeamMemberAddedActivityLogData) *TeamMemberAddedActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberAddedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Add member"),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogActionRemoved:
			data, err := activitylog.TransformData(entry, func(data *TeamMemberRemovedActivityLogData) *TeamMemberRemovedActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberRemovedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Remove member"),
				Data:                    data,
			}, nil
		case activityLogActionSetMemberRole:
			data, err := activitylog.TransformData(entry, func(data *TeamMemberSetRoleActivityLogData) *TeamMemberSetRoleActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return TeamMemberSetRoleActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Set member role"),
				Data:                    data,
			}, nil
		case activityLogActionUpdateEnvironment:
			data, err := activitylog.TransformData(entry, func(data *TeamEnvironmentUpdatedActivityLogData) *TeamEnvironmentUpdatedActivityLogData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamEnvironmentUpdatedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Update environment"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported team activity log entry action: %q", entry.Action)
		}
	})
}

type TeamCreatedActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type TeamUpdatedActivityLog struct {
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

type TeamConfirmDeleteKeyActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type TeamCreateDeleteKeyActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type TeamMemberAddedActivityLog struct {
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

type TeamMemberRemovedActivityLog struct {
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

type TeamMemberSetRoleActivityLog struct {
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

type TeamEnvironmentUpdatedActivityLog struct {
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
