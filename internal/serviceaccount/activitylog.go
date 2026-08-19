package serviceaccount

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
)

const (
	ActivityLogEntryResourceTypeServiceAccount                activitylog.ActivityLogEntryResourceType = "SERVICE_ACCOUNT"
	activityLogEntryActionAssignServiceAccountRole            activitylog.ActivityLogEntryAction       = "ASSIGN_SERVICE_ACCOUNT_TOKEN_ROLE"
	activityLogEntryActionRevokeServiceAccountRole            activitylog.ActivityLogEntryAction       = "REVOKE_SERVICE_ACCOUNT_TOKEN_ROLE"
	activityLogEntryActionCreateServiceAccountToken           activitylog.ActivityLogEntryAction       = "CREATE_SERVICE_ACCOUNT_TOKEN"
	activityLogEntryActionUpdateServiceAccountToken           activitylog.ActivityLogEntryAction       = "UPDATE_SERVICE_ACCOUNT_TOKEN"
	activityLogEntryActionDeleteServiceAccountToken           activitylog.ActivityLogEntryAction       = "DELETE_SERVICE_ACCOUNT_TOKEN"
	activityLogEntryActionAddServiceAccountWorkloadBinding    activitylog.ActivityLogEntryAction       = "ADD_SERVICE_ACCOUNT_WORKLOAD_BINDING"
	activityLogEntryActionRemoveServiceAccountWorkloadBinding activitylog.ActivityLogEntryAction       = "REMOVE_SERVICE_ACCOUNT_WORKLOAD_BINDING"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeServiceAccount, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
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
					return &ServiceAccountTokenUpdatedActivityLogEntryData{TokenName: data.TokenName}
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
		case activityLogEntryActionAddServiceAccountWorkloadBinding:
			data, err := activitylog.TransformData(entry, func(data *ServiceAccountWorkloadBindingAddedActivityLogEntryData) *ServiceAccountWorkloadBindingAddedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ServiceAccountWorkloadBindingAddedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Added workload to service account"),
				Data:                    data,
			}, nil
		case activityLogEntryActionRemoveServiceAccountWorkloadBinding:
			data, err := activitylog.TransformData(entry, func(data *ServiceAccountWorkloadBindingRemovedActivityLogEntryData) *ServiceAccountWorkloadBindingRemovedActivityLogEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return ServiceAccountWorkloadBindingRemovedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Removed workload from service account"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported service account activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_CREATED",
		activitylog.ActivityLogEntryActionCreated,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a service account is created."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_UPDATED",
		activitylog.ActivityLogEntryActionUpdated,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a service account is updated."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_DELETED",
		activitylog.ActivityLogEntryActionDeleted,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a service account is deleted."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_TOKEN_CREATED",
		activityLogEntryActionCreateServiceAccountToken,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a service account token is created."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_TOKEN_UPDATED",
		activityLogEntryActionUpdateServiceAccountToken,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a service account token is updated."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_TOKEN_DELETED",
		activityLogEntryActionDeleteServiceAccountToken,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a service account token is deleted."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_ROLE_ASSIGNED",
		activityLogEntryActionAssignServiceAccountRole,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a role is assigned to a service account."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_ROLE_REVOKED",
		activityLogEntryActionRevokeServiceAccountRole,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a role is revoked from a service account."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_WORKLOAD_BINDING_ADDED",
		activityLogEntryActionAddServiceAccountWorkloadBinding,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a workload binding is added to a service account."),
	)
	activitylog.RegisterActivityType(
		"SERVICE_ACCOUNT_WORKLOAD_BINDING_REMOVED",
		activityLogEntryActionRemoveServiceAccountWorkloadBinding,
		ActivityLogEntryResourceTypeServiceAccount,
		activitylog.WithDescription("Triggered when a workload binding is removed from a service account."),
	)
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
	// Nil for entries written before this field existed.
	TokenName     *string                                                       `json:"tokenName,omitempty"`
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

type ServiceAccountWorkloadBindingAddedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountWorkloadBindingAddedActivityLogEntryData `json:"data"`
}

type ServiceAccountWorkloadBindingAddedActivityLogEntryData struct {
	TeamSlug     slug.Slug      `json:"teamSlug"`
	WorkloadName string         `json:"workloadName"`
	WorkloadType *workload.Type `json:"workloadType,omitempty"`
}

type ServiceAccountWorkloadBindingRemovedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ServiceAccountWorkloadBindingRemovedActivityLogEntryData `json:"data"`
}

type ServiceAccountWorkloadBindingRemovedActivityLogEntryData struct {
	TeamSlug     slug.Slug      `json:"teamSlug"`
	WorkloadName string         `json:"workloadName"`
	WorkloadType *workload.Type `json:"workloadType,omitempty"`
}
