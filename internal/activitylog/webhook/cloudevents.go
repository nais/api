package webhook

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
)

// CloudEvent represents a CloudEvents 1.0 envelope.
type CloudEvent struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Source          string          `json:"source"`
	Type            string          `json:"type"`
	Subject         string          `json:"subject,omitempty"`
	Time            string          `json:"time"`
	DataContentType string          `json:"datacontenttype"`
	Data            json.RawMessage `json:"data"`
}

// CloudEventData is the data payload within a CloudEvent.
type CloudEventData struct {
	ActivityLogEntryID *uuid.UUID      `json:"activityLogEntryId,omitempty"`
	TriggeredAt        time.Time       `json:"triggeredAt"`
	Test               bool            `json:"test,omitempty"`
	Actor              string          `json:"actor"`
	ResourceType       string          `json:"resourceType"`
	ResourceName       string          `json:"resourceName"`
	TeamSlug           *string         `json:"teamSlug,omitempty"`
	Environment        *string         `json:"environment,omitempty"`
	Data               json.RawMessage `json:"data,omitempty"`
}

// cloudEventTypeFromEvent returns the CloudEvents-spec type for an event.
// If the event has resolved ActivityTypes, the first one is used; otherwise the raw
// event type is lowercased and dot-separated.
func cloudEventTypeFromEvent(event WebhookEvent) string {
	if len(event.ActivityTypes) > 0 {
		return activitylog.CloudEventType(activitylog.ActivityLogActivityType(event.ActivityTypes[0]))
	}
	// Synthetic test events use the same io.nais prefix.
	lower := strings.ToLower(event.RawEventType)
	dotted := strings.ReplaceAll(lower, "_", ".")
	return "io.nais." + dotted
}

// BuildCloudEvent creates a CloudEvents 1.0 envelope from a webhook event.
//
// id must be stable across redeliveries of the same logical (event, subscriber) delivery
// attempt.
func BuildCloudEvent(source, id string, event WebhookEvent) ([]byte, error) {
	if event.TriggeredAt.IsZero() {
		event.TriggeredAt = time.Now().UTC()
	}
	var teamSlug *string
	if event.TeamSlug != nil {
		s := event.TeamSlug.String()
		teamSlug = &s
	}

	eventData := CloudEventData{
		ActivityLogEntryID: event.ActivityLogEntryID,
		TriggeredAt:        event.TriggeredAt.UTC(),
		Test:               event.Test,
		Actor:              event.Actor,
		ResourceType:       event.ResourceType,
		ResourceName:       event.ResourceName,
		TeamSlug:           teamSlug,
		Environment:        event.Environment,
		Data:               event.Data,
	}

	dataBytes, err := json.Marshal(eventData)
	if err != nil {
		return nil, err
	}

	subject := event.ResourceName
	if event.TeamSlug != nil {
		subject = event.TeamSlug.String() + "/" + event.ResourceName
	}

	ce := CloudEvent{
		SpecVersion:     "1.0",
		ID:              id,
		Source:          source,
		Type:            cloudEventTypeFromEvent(event),
		Subject:         subject,
		Time:            event.TriggeredAt.UTC().Format(time.RFC3339Nano),
		DataContentType: "application/json",
		Data:            dataBytes,
	}

	return json.Marshal(ce)
}
