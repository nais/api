package webhook

import (
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
)

type WebhookSubscription struct {
	UUID                uuid.UUID   `json:"id"`
	Name                string      `json:"name"`
	TeamSlug            *slug.Slug  `json:"teamSlug,omitempty"`
	SourceTeamSlugs     []slug.Slug `json:"sourceTeamSlugs"`
	URL                 string      `json:"url"`
	Secret              string      `json:"-"`
	EventTypes          []string    `json:"eventTypes"`
	AllEvents           bool        `json:"allEvents"`
	Enabled             bool        `json:"enabled"`
	ConsecutiveFailures int         `json:"consecutiveFailures"`
	DisabledAt          *time.Time  `json:"disabledAt,omitempty"`
	CreatedAt           time.Time   `json:"createdAt"`
	UpdatedAt           time.Time   `json:"updatedAt"`
}

func (WebhookSubscription) IsNode() {}

func (WebhookSubscription) IsActivityLogger() {}

func (w WebhookSubscription) ID() ident.Ident {
	return newSubscriptionIdent(w.UUID)
}

type (
	WebhookSubscriptionConnection = pagination.Connection[*WebhookSubscription]
	WebhookSubscriptionEdge       = pagination.Edge[*WebhookSubscription]
)

type WebhookDelivery struct {
	UUID                  uuid.UUID             `json:"id"`
	SubscriptionID        uuid.UUID             `json:"subscriptionID"`
	DeliveryState         *WebhookDeliveryState `json:"deliveryState"`
	EventType             string                `json:"eventType"`
	IsTest                bool                  `json:"isTest"`
	RequestBody           string                `json:"requestBody"`
	RequestHeaders        []*WebhookHeader      `json:"requestHeaders"`
	ResponseStatus        *int                  `json:"responseStatus,omitempty"`
	ResponseBody          *string               `json:"responseBody,omitempty"`
	ResponseHeaders       []*WebhookHeader      `json:"responseHeaders"`
	ResponseBodyTruncated bool                  `json:"responseBodyTruncated"`
	ErrorMessage          *string               `json:"errorMessage,omitempty"`
	DurationMs            int                   `json:"durationMs"`
	Success               bool                  `json:"success"`
	CreatedAt             time.Time             `json:"createdAt"`
}

type WebhookHeader struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

type WebhookDeliveryState struct {
	Status       WebhookDeliveryStatus `json:"status"`
	AttemptCount int                   `json:"attemptCount"`
	NextRetryAt  *time.Time            `json:"nextRetryAt,omitempty"`
}

type WebhookDeliveryStatus string

const (
	WebhookDeliveryStatusCompleted WebhookDeliveryStatus = "COMPLETED"
	WebhookDeliveryStatusFailed    WebhookDeliveryStatus = "FAILED"
)

func (s WebhookDeliveryStatus) IsValid() bool {
	return slices.Contains([]WebhookDeliveryStatus{
		WebhookDeliveryStatusCompleted, WebhookDeliveryStatusFailed,
	}, s)
}

func (s WebhookDeliveryStatus) MarshalGQL(w io.Writer) {
	fmt.Fprintf(w, "%q", s)
}

func (s *WebhookDeliveryStatus) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("WebhookDeliveryStatus must be a string")
	}
	*s = WebhookDeliveryStatus(str)
	if !s.IsValid() {
		return fmt.Errorf("invalid WebhookDeliveryStatus: %q", str)
	}
	return nil
}

func (WebhookDelivery) IsNode() {}

func (w WebhookDelivery) ID() ident.Ident {
	return newDeliveryIdent(w.UUID)
}

type (
	WebhookDeliveryConnection = pagination.Connection[*WebhookDelivery]
	WebhookDeliveryEdge       = pagination.Edge[*WebhookDelivery]
)

type WebhookEventSelection struct {
	AllEvents  graphql.Omittable[*bool]    `json:"allEvents,omitzero"`
	EventTypes graphql.Omittable[[]string] `json:"eventTypes,omitzero"`
}

type CreateWebhookInput struct {
	Name            string                 `json:"name"`
	TeamSlug        *slug.Slug             `json:"teamSlug,omitempty"`
	SourceTeamSlugs []slug.Slug            `json:"sourceTeamSlugs,omitempty"`
	URL             string                 `json:"url"`
	Secret          string                 `json:"secret"`
	Events          *WebhookEventSelection `json:"events"`
}

type CreateWebhookPayload struct {
	Webhook *WebhookSubscription `json:"webhook"`
}

type UpdateWebhookInput struct {
	ID              ident.Ident            `json:"id"`
	Name            *string                `json:"name,omitempty"`
	SourceTeamSlugs []slug.Slug            `json:"sourceTeamSlugs,omitempty"`
	URL             *string                `json:"url,omitempty"`
	Secret          *string                `json:"secret,omitempty"`
	Events          *WebhookEventSelection `json:"events,omitempty"`
	Enabled         *bool                  `json:"enabled,omitempty"`
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

type TestWebhookInput struct {
	ID ident.Ident `json:"id"`
}

type TestWebhookPayload struct {
	Delivery *WebhookDelivery `json:"delivery"`
}

// WebhookEvent is an internal event passed to the dispatcher when an activity log entry is created.
type WebhookEvent struct {
	ActivityLogEntryID *uuid.UUID
	TriggeredAt        time.Time
	Test               bool
	// ActivityTypes holds the resolved ActivityLogActivityType values for this event
	// (e.g. ["TEAM_MEMBER_ADDED"]). Populated by the dispatcher via LookupActivityTypes.
	// For synthetic test events, this may be left nil.
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

func (e WebhookEvent) isIgnored() bool {
	for _, at := range e.ActivityTypes {
		if activitylog.IsIgnoredForWebhook(activitylog.ActivityLogActivityType(at)) {
			return true
		}
	}
	return false
}

func (w *WebhookSubscription) MatchesEvent(event WebhookEvent) bool {
	if !w.Enabled || event.isIgnored() {
		return false
	}

	if w.TeamSlug != nil {
		if event.TeamSlug == nil || (len(w.SourceTeamSlugs) > 0 && !slices.Contains(w.SourceTeamSlugs, *event.TeamSlug)) {
			return false
		}
		for _, at := range event.ActivityTypes {
			if !activitylog.IsTeamScoped(activitylog.ActivityLogActivityType(at)) {
				return false
			}
		}
	}

	if w.AllEvents {
		return true
	}
	for _, subType := range w.EventTypes {
		if slices.Contains(event.ActivityTypes, subType) {
			return true
		}
	}

	return false
}
