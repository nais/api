package webhook

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
)

func init() {
	activitylog.RegisterActivityType("WEBHOOK_TEST_CLOUDEVENTS_EVENT", activitylog.ActivityLogEntryActionCreated, "WEBHOOK_TEST_CLOUDEVENTS_RESOURCE")
}

func TestBuildCloudEvent_Envelope(t *testing.T) {
	team := slug.Slug("my-team")
	event := WebhookEvent{
		ActivityTypes: []string{"WEBHOOK_TEST_CLOUDEVENTS_EVENT"},
		TeamSlug:      &team,
		Actor:         "user@example.com",
		ResourceType:  "TEAM",
		ResourceName:  "my-resource",
	}

	before := time.Now().UTC()
	raw, err := BuildCloudEvent("https://api.example.com", "delivery-id-123", event)
	if err != nil {
		t.Fatalf("BuildCloudEvent() error = %v", err)
	}
	after := time.Now().UTC()

	var ce CloudEvent
	if err := json.Unmarshal(raw, &ce); err != nil {
		t.Fatalf("failed to unmarshal cloud event: %v", err)
	}

	if ce.SpecVersion != "1.0" {
		t.Errorf("SpecVersion = %q, want %q", ce.SpecVersion, "1.0")
	}
	if ce.ID != "delivery-id-123" {
		t.Errorf("ID = %q, want %q", ce.ID, "delivery-id-123")
	}
	if ce.Source != "https://api.example.com" {
		t.Errorf("Source = %q, want %q", ce.Source, "https://api.example.com")
	}
	if ce.DataContentType != "application/json" {
		t.Errorf("DataContentType = %q, want %q", ce.DataContentType, "application/json")
	}
	if ce.Subject != "my-team/my-resource" {
		t.Errorf("Subject = %q, want %q", ce.Subject, "my-team/my-resource")
	}
	wantType := activitylog.CloudEventType("WEBHOOK_TEST_CLOUDEVENTS_EVENT")
	if ce.Type != wantType {
		t.Errorf("Type = %q, want %q", ce.Type, wantType)
	}

	parsedTime, err := time.Parse(time.RFC3339, ce.Time)
	if err != nil {
		t.Fatalf("failed to parse Time as RFC3339: %v", err)
	}
	if parsedTime.Before(before.Add(-time.Second)) || parsedTime.After(after.Add(time.Second)) {
		t.Errorf("Time %v not within expected range [%v, %v]", parsedTime, before, after)
	}

	var data CloudEventData
	if err := json.Unmarshal(ce.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal cloud event data: %v", err)
	}
	if data.Actor != event.Actor {
		t.Errorf("Data.Actor = %q, want %q", data.Actor, event.Actor)
	}
	if data.ResourceType != event.ResourceType {
		t.Errorf("Data.ResourceType = %q, want %q", data.ResourceType, event.ResourceType)
	}
	if data.ResourceName != event.ResourceName {
		t.Errorf("Data.ResourceName = %q, want %q", data.ResourceName, event.ResourceName)
	}
	if data.TeamSlug == nil || *data.TeamSlug != team.String() {
		t.Errorf("Data.TeamSlug = %v, want %q", data.TeamSlug, team.String())
	}
	if data.Environment != nil {
		t.Errorf("Data.Environment = %v, want nil", data.Environment)
	}
}

func TestBuildCloudEvent_SubjectWithoutTeam(t *testing.T) {
	event := WebhookEvent{
		ActivityTypes: []string{"WEBHOOK_TEST_CLOUDEVENTS_EVENT"},
		ResourceName:  "my-resource",
	}

	raw, err := BuildCloudEvent("https://api.example.com", "id", event)
	if err != nil {
		t.Fatalf("BuildCloudEvent() error = %v", err)
	}

	var ce CloudEvent
	if err := json.Unmarshal(raw, &ce); err != nil {
		t.Fatalf("failed to unmarshal cloud event: %v", err)
	}

	if ce.Subject != "my-resource" {
		t.Errorf("Subject = %q, want %q", ce.Subject, "my-resource")
	}

	var data CloudEventData
	if err := json.Unmarshal(ce.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal cloud event data: %v", err)
	}
	if data.TeamSlug != nil {
		t.Errorf("Data.TeamSlug = %v, want nil", data.TeamSlug)
	}
}

func TestCloudEventTypeFromEvent_SyntheticEventFallsBackToRawType(t *testing.T) {
	event := WebhookEvent{
		RawEventType: "ping",
	}

	if got := cloudEventTypeFromEvent(event); got != "io.nais.ping" {
		t.Errorf("cloudEventTypeFromEvent() = %q, want %q", got, "io.nais.ping")
	}
}

func TestCloudEventTypeFromEvent_UsesFirstActivityType(t *testing.T) {
	event := WebhookEvent{
		ActivityTypes: []string{"WEBHOOK_TEST_CLOUDEVENTS_EVENT", "SOME_OTHER_TYPE"},
	}

	want := activitylog.CloudEventType("WEBHOOK_TEST_CLOUDEVENTS_EVENT")
	if got := cloudEventTypeFromEvent(event); got != want {
		t.Errorf("cloudEventTypeFromEvent() = %q, want %q", got, want)
	}
}

func TestBuildCloudEvent_DataOmittedWhenNil(t *testing.T) {
	event := WebhookEvent{
		ActivityTypes: []string{"WEBHOOK_TEST_CLOUDEVENTS_EVENT"},
		ResourceName:  "my-resource",
	}

	raw, err := BuildCloudEvent("https://api.example.com", "id", event)
	if err != nil {
		t.Fatalf("BuildCloudEvent() error = %v", err)
	}

	var ce CloudEvent
	if err := json.Unmarshal(raw, &ce); err != nil {
		t.Fatalf("failed to unmarshal cloud event: %v", err)
	}

	var raw2 map[string]json.RawMessage
	if err := json.Unmarshal(ce.Data, &raw2); err != nil {
		t.Fatalf("failed to unmarshal cloud event data as map: %v", err)
	}
	if _, ok := raw2["data"]; ok {
		t.Error("expected 'data' field to be omitted when event.Data is nil")
	}
	if _, ok := raw2["environment"]; ok {
		t.Error("expected 'environment' field to be omitted when event.Environment is nil")
	}
}
