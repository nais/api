package webhook

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/nais/api/internal/activitylog"
)

const ActivityLogEntryResourceTypeWebhook activitylog.ActivityLogEntryResourceType = "WEBHOOK"

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeWebhook, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			data, err := activitylog.UnmarshalData[WebhookActivityLogEntryData](entry)
			if err != nil {
				return nil, err
			}
			return WebhookCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Created webhook %q", data.Name)),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionUpdated:
			data, err := activitylog.UnmarshalData[WebhookUpdatedActivityLogEntryData](entry)
			if err != nil {
				return nil, err
			}
			return WebhookUpdatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Updated webhook %q", data.Name)),
				Data:                    data,
			}, nil
		case activitylog.ActivityLogEntryActionDeleted:
			data, err := activitylog.UnmarshalData[WebhookActivityLogEntryData](entry)
			if err != nil {
				return nil, err
			}
			return WebhookDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Deleted webhook %q", data.Name)),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported webhook activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterActivityType(
		"WEBHOOK_CREATED",
		activitylog.ActivityLogEntryActionCreated,
		ActivityLogEntryResourceTypeWebhook,
		"Triggered when a webhook subscription is created.",
	)
	activitylog.RegisterActivityType(
		"WEBHOOK_UPDATED",
		activitylog.ActivityLogEntryActionUpdated,
		ActivityLogEntryResourceTypeWebhook,
		"Triggered when a webhook subscription is updated.",
	)
	activitylog.RegisterActivityType(
		"WEBHOOK_DELETED",
		activitylog.ActivityLogEntryActionDeleted,
		ActivityLogEntryResourceTypeWebhook,
		"Triggered when a webhook subscription is deleted.",
	)
}

type WebhookCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *WebhookActivityLogEntryData `json:"data"`
}

type WebhookActivityLogEntryData struct {
	Name string `json:"name"`
}

type WebhookUpdatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *WebhookUpdatedActivityLogEntryData `json:"data"`
}

type WebhookUpdatedActivityLogEntryData struct {
	Name          string                                            `json:"name"`
	UpdatedFields []*WebhookUpdatedActivityLogEntryDataUpdatedField `json:"updatedFields"`
}

type WebhookUpdatedActivityLogEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue"`
	NewValue *string `json:"newValue"`
}

type WebhookDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *WebhookActivityLogEntryData `json:"data"`
}

func webhookUpdatedFields(before, after *WebhookSubscription) []*WebhookUpdatedActivityLogEntryDataUpdatedField {
	fields := make([]*WebhookUpdatedActivityLogEntryDataUpdatedField, 0)
	if before.Name != after.Name {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{
			Field:    "name",
			OldValue: &before.Name,
			NewValue: &after.Name,
		})
	}
	// Endpoint URLs can contain credentials. Log only the field names for URLs and signing secrets.
	if before.URL != after.URL {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{Field: "url"})
	}
	if before.Secret != after.Secret {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{Field: "secret"})
	}
	if !slices.Equal(before.EventTypes, after.EventTypes) {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{
			Field:    "eventTypes",
			OldValue: new(fmt.Sprint(before.EventTypes)),
			NewValue: new(fmt.Sprint(after.EventTypes)),
		})
	}
	if before.AllEvents != after.AllEvents {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{
			Field:    "allEvents",
			OldValue: new(strconv.FormatBool(before.AllEvents)),
			NewValue: new(strconv.FormatBool(after.AllEvents)),
		})
	}
	if !slices.Equal(before.SourceTeamSlugs, after.SourceTeamSlugs) {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{
			Field:    "sourceTeamSlugs",
			OldValue: new(fmt.Sprint(before.SourceTeamSlugs)),
			NewValue: new(fmt.Sprint(after.SourceTeamSlugs)),
		})
	}
	if before.Enabled != after.Enabled {
		fields = append(fields, &WebhookUpdatedActivityLogEntryDataUpdatedField{
			Field:    "enabled",
			OldValue: new(strconv.FormatBool(before.Enabled)),
			NewValue: new(strconv.FormatBool(after.Enabled)),
		})
	}
	return fields
}
