package serviceaccount

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeServiceAccount      activitylog.ActivityLogEntryResourceType = "SERVICE_ACCOUNT"
	activityLogEntryActionAssignServiceAccountRole  activitylog.ActivityLogEntryAction       = "ASSIGN_SERVICE_ACCOUNT_TOKEN_ROLE"
	activityLogEntryActionRevokeServiceAccountRole  activitylog.ActivityLogEntryAction       = "REVOKE_SERVICE_ACCOUNT_TOKEN_ROLE"
	activityLogEntryActionCreateServiceAccountToken activitylog.ActivityLogEntryAction       = "CREATE_SERVICE_ACCOUNT_TOKEN"
	activityLogEntryActionUpdateServiceAccountToken activitylog.ActivityLogEntryAction       = "UPDATE_SERVICE_ACCOUNT_TOKEN"
	activityLogEntryActionDeleteServiceAccountToken activitylog.ActivityLogEntryAction       = "DELETE_SERVICE_ACCOUNT_TOKEN"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeServiceAccount, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return ServiceAccountCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created service account"),
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.TransformData(entry, func(data *ServiceAccountUpdatedActivityLogEntryData) *ServiceAccountUpdatedActivityLogEntryData {
				if len(data.UpdatedFields) == 0 {
					return &ServiceAccountUpdatedActivityLogEntryData{}
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return ServiceAccountUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated service account"),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			return ServiceAccountDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted service account"),
			}, nil
		case activityLogEntryActionCreateServiceAccountToken:
			data, err := activitylog.TransformData(entry, func(data *ServiceAccountTokenCreatedActivityLogEntryData) *ServiceAccountTokenCreatedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ServiceAccountTokenCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created service account token"),
				Data:                    data,
			}, nil
		case activityLogEntryActionUpdateServiceAccountToken:
			data, err := activitylog.TransformData(entry, func(data *ServiceAccountTokenUpdatedActivityLogEntryData) *ServiceAccountTokenUpdatedActivityLogEntryData {
				if len(data.UpdatedFields) == 0 {
					return &ServiceAccountTokenUpdatedActivityLogEntryData{}
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return ServiceAccountTokenUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated service account token"),
				Data:                    data,
			}, nil
		case activityLogEntryActionDeleteServiceAccountToken:
			data, err := activitylog.TransformData(entry, func(data *ServiceAccountTokenDeletedActivityLogEntryData) *ServiceAccountTokenDeletedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ServiceAccountTokenDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Delete service account token"),
				Data:                    data,
			}, nil
		case activityLogEntryActionAssignServiceAccountRole:
			data, err := activitylog.TransformData(entry, func(data *RoleAssignedToServiceAccountActivityLogEntryData) *RoleAssignedToServiceAccountActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return RoleAssignedToServiceAccountActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Assigned role to service account"),
				Data:                    data,
			}, nil
		case activityLogEntryActionRevokeServiceAccountRole:
			data, err := activitylog.TransformData(entry, func(data *RoleRevokedFromServiceAccountActivityLogEntryData) *RoleRevokedFromServiceAccountActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return RoleRevokedFromServiceAccountActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Revoked role from service account"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported service account activity log entry action: %q", entry.Action)
		}
	})
}

type RoleAssignedToServiceAccountActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *RoleAssignedToServiceAccountActivityLogEntryData `json:"data"`
}

type RoleAssignedToServiceAccountActivityLogEntryData struct {
	RoleName string `json:"roleName"`
}

type RoleRevokedFromServiceAccountActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *RoleRevokedFromServiceAccountActivityLogEntryData `json:"data"`
}

type RoleRevokedFromServiceAccountActivityLogEntryData struct {
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
	TokenName string `json:"tokenName"`
}

type ServiceAccountTokenDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountTokenDeletedActivityLogEntryData `json:"data"`
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

type ServiceAccountTokenDeletedActivityLogEntryData struct {
	TokenName string `json:"tokenName"`
}
