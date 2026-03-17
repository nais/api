package aivencredentials

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryActionCreateCredentials activitylog.ActivityLogEntryAction = "CREATE_CREDENTIALS"

	activityLogEntryResourceTypeCredentials activitylog.ActivityLogEntryResourceType = "CREDENTIALS"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeCredentials, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionCreateCredentials:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for credentials activity log entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for credentials activity log entry")
			}
			data, err := activitylog.UnmarshalData[CredentialsActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming credentials activity log entry data: %w", err)
			}

			msg := fmt.Sprintf("Created %s credentials", data.ServiceType)
			if data.InstanceName != "" {
				msg += fmt.Sprintf(" for %s", data.InstanceName)
			}
			if data.Permission != "" {
				msg += fmt.Sprintf(" with %s permission", data.Permission)
			}
			msg += fmt.Sprintf(" (TTL: %s)", data.TTL)

			return CredentialsActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(msg),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported credentials activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("CREDENTIALS_CREATE", activityLogEntryActionCreateCredentials, activityLogEntryResourceTypeCredentials)
}

type CredentialsActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *CredentialsActivityLogEntryData `json:"data"`
}

type CredentialsActivityLogEntryData struct {
	ServiceType  string `json:"serviceType"`
	InstanceName string `json:"instanceName,omitempty"`
	Permission   string `json:"permission,omitempty"`
	TTL          string `json:"ttl"`
}
