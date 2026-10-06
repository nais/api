package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/nais/api/internal/activitylog"
)

func TestDeletePostgresInputValidation(t *testing.T) {
	for _, tt := range []struct {
		name  string
		input DeletePostgresInput
		want  string
	}{
		{"empty name", DeletePostgresInput{TeamSlug: "team", EnvironmentName: "dev"}, "name"},
		{"invalid name", DeletePostgresInput{Name: "UPPER", TeamSlug: "team", EnvironmentName: "dev"}, "name"},
		{"empty environment", DeletePostgresInput{Name: "db", TeamSlug: "team"}, "environmentName"},
		{"empty team", DeletePostgresInput{Name: "db", EnvironmentName: "dev"}, "teamSlug"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.input.Validate(context.Background()); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() = %v, want error for %s", err, tt.want)
			}
		})
	}
}

func TestPostgresDeletionActivityMessages(t *testing.T) {
	for _, tt := range []struct {
		name   string
		action activitylog.ActivityLogEntryAction
		want   string
	}{
		{"historical branch deletion", activitylog.ActivityLogEntryActionDeleted, "Deleted Postgres branch (name unavailable)"},
		{"whole Postgres deletion request", activityLogEntryActionDeletionRequested, "Requested deletion of Postgres; cleanup is pending"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := transformPostgresActivityLogEntry(activitylog.GenericActivityLogEntry{Action: tt.action})
			if err != nil {
				t.Fatal(err)
			}
			deleted, ok := entry.(PostgresDeletedActivityLogEntry)
			if !ok || deleted.Message != tt.want {
				t.Fatalf("entry = %#v, want PostgresDeletedActivityLogEntry with message %q", entry, tt.want)
			}
		})
	}
}
