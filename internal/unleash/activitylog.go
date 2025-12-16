package unleash

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
)

const (
	activityLogEntryResourceTypeUnleash activitylog.ActivityLogEntryResourceType = "UNLEASH"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeUnleash, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return UnleashInstanceCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Created Unleash instance"),
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.TransformData(entry, func(data *UnleashInstanceUpdatedActivityLogEntryData) *UnleashInstanceUpdatedActivityLogEntryData {
				if data.AllowedTeamSlug == nil && data.RevokedTeamSlug == nil && data.UpdatedReleaseChannel == nil {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return UnleashInstanceUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Updated Unleash instance"),
				Data:                    data,
			}, nil

		default:
			return nil, fmt.Errorf("unsupported unleash activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("UNLEASH_INSTANCE_CREATED", activitylog.ActivityLogEntryActionCreated, activityLogEntryResourceTypeUnleash)
	activitylog.RegisterFilter("UNLEASH_INSTANCE_UPDATED", activitylog.ActivityLogEntryActionUpdated, activityLogEntryResourceTypeUnleash)
}

type UnleashInstanceCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type UnleashInstanceUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *UnleashInstanceUpdatedActivityLogEntryData `json:"data"`
}

type UnleashInstanceUpdatedActivityLogEntryData struct {
	RevokedTeamSlug       *slug.Slug `json:"revokedTeamSlug"`
	AllowedTeamSlug       *slug.Slug `json:"allowedTeamSlug"`
	UpdatedReleaseChannel *string    `json:"updatedReleaseChannel,omitempty"`
}
