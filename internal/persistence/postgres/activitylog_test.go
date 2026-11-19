package postgres

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/activitylog"
)

func TestHistoricalPostgresActivity(t *testing.T) {
	original := `{"retiredResource":"main","targetTime":"2026-01-01T00:00:00Z"}`
	payload := &PostgresHistoricalActivityLogEntryData{OriginalAction: "RETIRED_OPERATION", OriginalData: &original}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	entry := activitylog.GenericActivityLogEntry{Action: activityLogEntryActionHistorical, ResourceName: "orders", CreatedAt: created, Data: data}
	got, err := transformPostgresActivityLogEntry(entry)
	if err != nil {
		t.Fatal(err)
	}
	historical, ok := got.(PostgresHistoricalActivityLogEntry)
	if !ok {
		t.Fatalf("got %T, want historical event", got)
	}
	if diff := cmp.Diff(payload, historical.Data); diff != "" {
		t.Errorf("payload (-want +got):\n%s", diff)
	}
	if historical.ResourceName != entry.ResourceName || !historical.CreatedAt.Equal(created) {
		t.Error("audit identity changed")
	}
	if historical.Message != "Historical Postgres operation: RETIRED_OPERATION" {
		t.Errorf("message = %q", historical.Message)
	}
}

func TestHistoricalPostgresActivityInvalidPayload(t *testing.T) {
	_, err := transformPostgresActivityLogEntry(activitylog.GenericActivityLogEntry{Action: activityLogEntryActionHistorical, Data: []byte("not json")})
	if err == nil {
		t.Fatal("accepted malformed historical payload")
	}
}
