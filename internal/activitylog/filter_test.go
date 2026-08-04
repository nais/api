package activitylog

import (
	"slices"
	"testing"
)

func TestRegisterActivityType_PanicsOnConflictingAction(t *testing.T) {
	const activityType ActivityLogActivityType = "FILTER_TEST_CONFLICT"

	RegisterActivityType(activityType, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_A")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected RegisterActivityType to panic when re-registering with a different action")
		}
	}()

	RegisterActivityType(activityType, ActivityLogEntryActionUpdated, "FILTER_TEST_RESOURCE_A")
}

func TestRegisterActivityType_MergesResourceTypes(t *testing.T) {
	const activityType ActivityLogActivityType = "FILTER_TEST_MERGE"

	RegisterActivityType(activityType, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_B")
	RegisterActivityType(activityType, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_C")
	// Registering the same resource type again must not create a duplicate entry.
	RegisterActivityType(activityType, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_B")

	typesB := LookupActivityTypes("FILTER_TEST_RESOURCE_B", string(ActivityLogEntryActionCreated))
	typesC := LookupActivityTypes("FILTER_TEST_RESOURCE_C", string(ActivityLogEntryActionCreated))

	if !slices.Contains(typesB, activityType) {
		t.Fatalf("expected %q to be resolvable from resource B, got %v", activityType, typesB)
	}
	if !slices.Contains(typesC, activityType) {
		t.Fatalf("expected %q to be resolvable from resource C, got %v", activityType, typesC)
	}

	f := knownFilters[activityType]
	count := 0
	for _, rt := range f.resourceType {
		if rt == "FILTER_TEST_RESOURCE_B" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected FILTER_TEST_RESOURCE_B to appear exactly once, got %d", count)
	}
}

func TestKnownEventTypes_ExcludesIgnoreWebhook(t *testing.T) {
	const visible ActivityLogActivityType = "FILTER_TEST_VISIBLE"
	const hidden ActivityLogActivityType = "FILTER_TEST_HIDDEN"

	RegisterActivityType(visible, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_D")
	RegisterActivityType(hidden, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_D", IgnoreWebhook())

	all := KnownEventTypes()

	foundVisible := false
	for _, info := range all {
		if info.Type == hidden {
			t.Fatalf("expected %q to be excluded from KnownEventTypes, but it was present", hidden)
		}
		if info.Type == visible {
			foundVisible = true
		}
	}
	if !foundVisible {
		t.Fatalf("expected %q to be present in KnownEventTypes", visible)
	}
}

func TestIsTeamScoped(t *testing.T) {
	const teamScoped ActivityLogActivityType = "FILTER_TEST_TEAM_SCOPED"
	const globalOnly ActivityLogActivityType = "FILTER_TEST_GLOBAL_ONLY"

	RegisterActivityType(teamScoped, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_E")
	RegisterActivityType(globalOnly, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_E", GlobalOnly())

	if !IsTeamScoped(teamScoped) {
		t.Errorf("expected %q to be team-scoped by default", teamScoped)
	}
	if IsTeamScoped(globalOnly) {
		t.Errorf("expected %q to be global-only", globalOnly)
	}
	if !IsTeamScoped("FILTER_TEST_UNKNOWN") {
		t.Error("expected an unregistered activity type to default to team-scoped")
	}
}

func TestIsValidActivityType(t *testing.T) {
	const activityType ActivityLogActivityType = "FILTER_TEST_VALID"
	RegisterActivityType(activityType, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_F")

	if !IsValidActivityType("*") {
		t.Error(`expected "*" to be valid`)
	}
	if !IsValidActivityType(string(activityType)) {
		t.Errorf("expected %q to be valid", activityType)
	}
	if IsValidActivityType("FILTER_TEST_NOT_REGISTERED") {
		t.Error("expected an unregistered activity type to be invalid")
	}
}

func TestCloudEventType(t *testing.T) {
	tests := []struct {
		in   ActivityLogActivityType
		want string
	}{
		{"TEAM_MEMBER_ADDED", "io.nais.team.member.added"},
		{"POSTGRES_DELETED", "io.nais.postgres.deleted"},
	}

	for _, tt := range tests {
		if got := CloudEventType(tt.in); got != tt.want {
			t.Errorf("CloudEventType(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAutoGroupAndDescription(t *testing.T) {
	tests := []struct {
		in        ActivityLogActivityType
		wantGroup string
	}{
		{"TEAM_MEMBER_ADDED", "Team"},
		{"SERVICE_ACCOUNT_CREATED", "Service Account"},
		{"GENERIC_KUBERNETES_RESOURCE_CREATED", "Kubernetes"},
		{"OPENSEARCH_CREATED", "OpenSearch"},
		{"JOB_RUN_DELETED", "Job"},
	}

	for _, tt := range tests {
		_, group := autoGroupAndDescription(tt.in)
		if group != tt.wantGroup {
			t.Errorf("autoGroupAndDescription(%q) group = %q, want %q", tt.in, group, tt.wantGroup)
		}
	}
}

func TestRegisterActivityType_CustomOptions(t *testing.T) {
	const activityType ActivityLogActivityType = "FILTER_TEST_CUSTOM"

	RegisterActivityType(activityType, ActivityLogEntryActionCreated, "FILTER_TEST_RESOURCE_G",
		WithDescription("custom description"),
		WithGroup("Custom Group"),
	)

	var found *WebhookEventTypeInfo
	for _, info := range KnownEventTypes() {
		if info.Type == activityType {
			info := info
			found = &info
			break
		}
	}
	if found == nil {
		t.Fatalf("expected %q to be present in KnownEventTypes", activityType)
	}
	if found.Description != "custom description" {
		t.Errorf("expected custom description, got %q", found.Description)
	}
	if found.Group != "Custom Group" {
		t.Errorf("expected custom group, got %q", found.Group)
	}
	if !found.TeamScoped {
		t.Error("expected TeamScoped to default to true")
	}
}

func TestLookupActivityTypes_UnknownReturnsEmpty(t *testing.T) {
	if got := LookupActivityTypes("FILTER_TEST_UNKNOWN_RESOURCE", "UNKNOWN_ACTION"); len(got) != 0 {
		t.Errorf("expected no matches, got %v", got)
	}
}
