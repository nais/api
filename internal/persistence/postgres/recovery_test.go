package postgres

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/kubernetes"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRecoveryInputValidation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		recovery  *PostgresRecoveryInput
		wantError string
	}{
		{name: "empty database"},
		{name: "valid recovery", recovery: &PostgresRecoveryInput{SourcePostgres: "source", TargetTime: time.Now().UTC().Truncate(time.Second).Add(-time.Hour)}},
		{name: "source required", recovery: &PostgresRecoveryInput{TargetTime: time.Now().Add(-time.Hour).Truncate(time.Second)}, wantError: "recovery.sourcePostgres"},
		{name: "different name", recovery: &PostgresRecoveryInput{SourcePostgres: "restored", TargetTime: time.Now().Add(-time.Hour).Truncate(time.Second)}, wantError: "different name"},
		{name: "explicit target", recovery: &PostgresRecoveryInput{SourcePostgres: "source"}, wantError: "recovery.targetTime"},
		{name: "future target", recovery: &PostgresRecoveryInput{SourcePostgres: "source", TargetTime: time.Now().Add(time.Hour)}, wantError: "no later than now"},
		{name: "fractional target", recovery: &PostgresRecoveryInput{SourcePostgres: "source", TargetTime: time.Unix(1_750_000_000, 1)}, wantError: "whole-second"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			input := CreatePostgresInput{Name: "restored", EnvironmentName: "dev", TeamSlug: "team", MajorVersion: "18", Recovery: tt.recovery}
			err := input.ValidationErrors(t.Context()).NilIfEmpty()
			if tt.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestRecoveryMetadataSurvivesWithoutSourceLookup(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(map[bool]string{false: "pending", true: "completed"}[completed], func(t *testing.T) {
			pg := &nais_io_v1.Postgres{ObjectMeta: metav1.ObjectMeta{Name: "restored", Namespace: "team"}, Spec: nais_io_v1.PostgresSpec{MajorVersion: "18", Recovery: &nais_io_v1.PostgresRecovery{SourcePostgres: "deleted-source", TargetTime: metav1.NewTime(time.Unix(1_750_000_000, 0))}}}
			if completed {
				pg.Status = &nais_io_v1.PostgresStatus{Ownership: &nais_io_v1.PostgresOwnership{ClusterUID: "original-cluster"}}
				pg.GetStatus().SetCondition(metav1.Condition{Type: "RecoveryCompleted", Status: metav1.ConditionTrue, Message: "original-cluster"})
			}
			object, err := kubernetes.ToUnstructured(pg)
			if err != nil {
				t.Fatal(err)
			}
			got, err := toPostgres(object, "dev")
			if err != nil {
				t.Fatal(err)
			}
			if got.Recovery == nil || got.Recovery.SourcePostgres != "deleted-source" || got.Recovery.Completed != completed || !got.Recovery.TargetTime.Equal(pg.Spec.Recovery.TargetTime.Time) {
				t.Fatalf("lost recovery provenance: %#v", got.Recovery)
			}
		})
	}
}

func TestRecoveryCreationAuditPreservesHistoricalEvents(t *testing.T) {
	for _, recovery := range []*PostgresRecoveryInput{nil, {SourcePostgres: "source", TargetTime: time.Date(2026, 9, 30, 14, 0, 0, 0, time.FixedZone("CEST", 7200))}} {
		entry := activitylog.GenericActivityLogEntry{Action: activitylog.ActivityLogEntryActionCreated}
		want := "Created Postgres"
		if recovery != nil {
			var err error
			entry.Data, err = json.Marshal(PostgresCreatedActivityLogEntryData{Recovery: recovery})
			if err != nil {
				t.Fatal(err)
			}
			want = "Requested Postgres recovery from source to 2026-09-30T12:00:00Z"
		}
		got, err := transformPostgresActivityLogEntry(entry)
		if err != nil {
			t.Fatal(err)
		}
		if got.(PostgresCreatedActivityLogEntry).Message != want {
			t.Fatalf("audit message = %q, want %q", got.(PostgresCreatedActivityLogEntry).Message, want)
		}
	}
}
