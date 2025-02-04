package serviceaccount

import (
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/scalar"
)

const (
	activityLogEntryResourceTypeServiceAccount activitylog.ActivityLogEntryResourceType = "SERVICE_ACCOUNT"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeServiceAccount, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		/*
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

		*/
		panic("implement")
	})
}

type RoleAddedToServiceAccountActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *RoleAddedToServiceAccountActivityLogEntryData `json:"data"`
}

type RoleAddedToServiceAccountActivityLogEntryData struct {
	RoleName string `json:"roleName"`
}

type RoleRemovedFromServiceAccountActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *RoleRemovedFromServiceAccountActivityLogEntryData `json:"data"`
}

type RoleRemovedFromServiceAccountActivityLogEntryData struct {
	RoleName string `json:"roleName"`
}

type ServiceAccountCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ServiceAccountDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ServiceAccountTokenCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountTokenCreatedActivityLogEntryData `json:"data"`
}

type ServiceAccountTokenCreatedActivityLogEntryData struct {
	Note      string       `json:"note"`
	ExpiresAt *scalar.Date `json:"expiresAt,omitempty"`
}

type ServiceAccountTokenDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountTokenDeletedActivityLogEntryData `json:"data"`
}

type ServiceAccountTokenDeletedActivityLogEntryData struct {
	Note string `json:"note"`
}

type ServiceAccountTokenUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountTokenUpdatedActivityLogEntryData `json:"data"`
}

type ServiceAccountTokenUpdatedActivityLogEntryData struct {
	UpdatedFields []*ServiceAccountTokenUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type ServiceAccountTokenUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}

type ServiceAccountUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountUpdatedActivityLogEntryData `json:"data"`
}

type ServiceAccountUpdatedActivityLogEntryData struct {
	UpdatedFields []*ServiceAccountUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type ServiceAccountUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}
