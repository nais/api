package webhook

import (
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type WebhookSubscription struct {
	UUID                uuid.UUID  `json:"id"`
	TeamSlug            *slug.Slug `json:"teamSlug,omitempty"`
	URL                 string     `json:"url"`
	Secret              string     `json:"-"`
	EventTypes          []string   `json:"eventTypes"`
	Enabled             bool       `json:"enabled"`
	ConsecutiveFailures int        `json:"consecutiveFailures"`
	DisabledAt          *time.Time `json:"disabledAt,omitempty"`
	CreatedBy           string     `json:"createdBy"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
}

func (WebhookSubscription) IsNode() {}

func (w WebhookSubscription) GetID() ident.Ident {
	return newSubscriptionIdent(w.UUID)
}

type (
	WebhookSubscriptionConnection = pagination.Connection[*WebhookSubscription]
	WebhookSubscriptionEdge       = pagination.Edge[*WebhookSubscription]
)

type WebhookDelivery struct {
	UUID           uuid.UUID `json:"id"`
	SubscriptionID uuid.UUID `json:"subscriptionID"`
	EventType      string    `json:"eventType"`
	RequestBody    string    `json:"requestBody"`
	ResponseStatus *int      `json:"responseStatus,omitempty"`
	ResponseBody   *string   `json:"responseBody,omitempty"`
	DurationMs     int       `json:"durationMs"`
	Success        bool      `json:"success"`
	CreatedAt      time.Time `json:"createdAt"`
}

func (WebhookDelivery) IsNode() {}

func (w WebhookDelivery) GetID() ident.Ident {
	return newDeliveryIdent(w.UUID)
}

type (
	WebhookDeliveryConnection = pagination.Connection[*WebhookDelivery]
	WebhookDeliveryEdge       = pagination.Edge[*WebhookDelivery]
)

type CreateWebhookInput struct {
	TeamSlug   *slug.Slug `json:"teamSlug,omitempty"`
	URL        string     `json:"url"`
	Secret     string     `json:"secret"`
	EventTypes []string   `json:"eventTypes"`
}

type CreateWebhookPayload struct {
	Webhook *WebhookSubscription `json:"webhook"`
}

type UpdateWebhookInput struct {
	ID         ident.Ident `json:"id"`
	URL        *string     `json:"url,omitempty"`
	Secret     *string     `json:"secret,omitempty"`
	EventTypes []string    `json:"eventTypes,omitempty"`
	Enabled    *bool       `json:"enabled,omitempty"`
}

type UpdateWebhookPayload struct {
	Webhook *WebhookSubscription `json:"webhook"`
}

type DeleteWebhookInput struct {
	ID ident.Ident `json:"id"`
}

type DeleteWebhookPayload struct {
	WebhookID ident.Ident `json:"webhookID"`
}

// WebhookEvent is an internal event passed to the dispatcher when an activity log entry is created.
type WebhookEvent struct {
	// ActivityTypes holds the resolved ActivityLogActivityType values for this event
	// (e.g. ["TEAM_MEMBER_ADDED"]). Populated by the dispatcher via LookupActivityTypes.
	// For synthetic events such as "ping", this may be left nil.
	ActivityTypes []string
	// RawEventType is the raw "RESOURCE_TYPE:ACTION" string stored in the outbox.
	RawEventType string
	TeamSlug     *slug.Slug
	Actor        string
	ResourceType string
	ResourceName string
	Environment  *string
	Data         []byte
}

func (w *WebhookSubscription) MatchesEvent(event WebhookEvent) bool {
	if !w.Enabled {
		return false
	}

	// Global webhooks (no team) match all events.
	// Team-scoped webhooks only match events for that team.
	if w.TeamSlug != nil {
		if event.TeamSlug == nil || *w.TeamSlug != *event.TeamSlug {
			return false
		}
	}

	for _, subType := range w.EventTypes {
		if subType == "*" {
			return true
		}
		for _, at := range event.ActivityTypes {
			if subType == at {
				return true
			}
		}
	}

	return false
}

// Node interface compatibility
func (w WebhookSubscription) ID() ident.Ident { return w.GetID() }
func (w WebhookDelivery) ID() ident.Ident     { return w.GetID() }
