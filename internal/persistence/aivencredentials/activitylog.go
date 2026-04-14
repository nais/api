package aivencredentials

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogActivityTypeCredentialsCreated activitylog.ActivityLogActivityType = "CREDENTIALS_CREATED"
	ActivityLogEntryActionCredentialsCreated  activitylog.ActivityLogEntryAction  = "CREDENTIALS_CREATED"
)

func GetActivityLogEntry(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
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

	msg := fmt.Sprintf("Created %s credentials for %s", entry.ResourceType, entry.ResourceName)
	if data.Permission != "" {
		msg += fmt.Sprintf(" with %s permission", data.Permission)
	}
	msg += fmt.Sprintf(" (TTL: %s)", data.TTL)

	return CredentialsActivityLogEntry{
		GenericActivityLogEntry: entry.WithMessage(msg),
		Data:                    data,
	}, nil
}

type CredentialsActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *CredentialsActivityLogEntryData `json:"data"`
}

type CredentialsActivityLogEntryData struct {
	Permission string `json:"permission,omitempty"`
	TTL        string `json:"ttl"`
}
