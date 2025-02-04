package serviceaccount

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/scalar"
)

const (
	activityLogEntryResourceTypeServiceAccount activitylog.ActivityLogEntryResourceType = "SERVICE_ACCOUNT"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeServiceAccount, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return ServiceAccountCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created service account"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported service account activity log entry action: %q", entry.Action)
		}
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
