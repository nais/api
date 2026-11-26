package postgres

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/pgrator/pkg/api"
	naisv1 "github.com/nais/pgrator/pkg/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPostgresBackupFilterMatches(t *testing.T) {
	backup := &PostgresBackup{Name: "newest", SourcePostgres: "deleted-db", EnvironmentName: "dev"}
	filter := &PostgresBackupFilter{
		Environments:   []string{"dev"},
		SourcePostgres: new("deleted-db"),
	}
	if !filter.Matches(backup) {
		t.Fatal("expected matching backup to be included")
	}
	if filter.Matches(&PostgresBackup{Name: "other-env", SourcePostgres: "deleted-db", EnvironmentName: "prod"}) {
		t.Fatal("expected backup from another environment to be filtered out")
	}
	if filter.Matches(&PostgresBackup{Name: "other-source", SourcePostgres: "another-db", EnvironmentName: "dev"}) {
		t.Fatal("expected backup from another source database to be filtered out")
	}
}

func TestPostgresBackupPublicStatus(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	disk := resource.MustParse("20Gi")
	for _, tc := range []struct {
		name    string
		status  *naisv1.PostgresBackupStatus
		phase   PostgresBackupPhase
		wantErr string
	}{
		{"new request", nil, PostgresBackupPhasePreparing, ""},
		{"copying", &naisv1.PostgresBackupStatus{Phase: naisv1.PostgresBackupPhaseRunning}, PostgresBackupPhaseRunning, ""},
		{"completed with snapshot and a size above 32 bits", &naisv1.PostgresBackupStatus{
			Phase:        naisv1.PostgresBackupPhaseCompleted,
			MajorVersion: "17",
			DiskSize:     &disk,
			Extensions:   []naisv1.PostgresExtension{{Name: "vector"}},
			CompletedAt:  new(metav1.NewTime(now)), ExpiresAt: new(metav1.NewTime(now.Add(30 * 24 * time.Hour))), SizeBytes: new(int64(5 << 30)),
		}, PostgresBackupPhaseCompleted, ""},
		{"failed base backup", &naisv1.PostgresBackupStatus{
			BaseStatus:   api.BaseStatus{Conditions: []metav1.Condition{{Type: "Ready", Reason: "BackupFailed"}}},
			Phase:        naisv1.PostgresBackupPhaseFailed,
			ErrorMessage: "cnpg internal error",
		}, PostgresBackupPhaseFailed, "The database base backup failed. Check that the source database is healthy and try again."},
		{"failed copy", &naisv1.PostgresBackupStatus{
			BaseStatus:   api.BaseStatus{Conditions: []metav1.Condition{{Type: "Ready", Reason: "CopyFailed"}}},
			Phase:        naisv1.PostgresBackupPhaseFailed,
			ErrorMessage: "gs://internal-bucket/prefix failed",
		}, PostgresBackupPhaseFailed, "Copying or verifying the standalone backup failed. Contact the Nais team if the problem persists."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			obj, err := kubernetes.ToUnstructured(&naisv1.PostgresBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "dr", Namespace: "team"}, Spec: naisv1.PostgresBackupSpec{Postgres: "source"}, Status: tc.status,
			})
			if err != nil {
				t.Fatal(err)
			}
			got, err := toPostgresBackup(obj, "dev")
			if err != nil {
				t.Fatal(err)
			}
			if got.Phase != tc.phase || got.RetentionDays != 30 || got.SourcePostgres != "source" {
				t.Errorf("unexpected backup: %+v", got)
			}
			if tc.phase == PostgresBackupPhaseCompleted {
				if diff := cmp.Diff(new(int64(5<<30)), got.SizeBytes); diff != "" {
					t.Errorf("backup size (-want +got): %s", diff)
				}
				if diff := cmp.Diff(new("17"), got.MajorVersion); diff != "" {
					t.Errorf("majorVersion (-want +got): %s", diff)
				}
				if diff := cmp.Diff(new("20Gi"), got.DiskSize); diff != "" {
					t.Errorf("diskSize (-want +got): %s", diff)
				}
				if diff := cmp.Diff([]string{"vector"}, got.Extensions); diff != "" {
					t.Errorf("extensions (-want +got): %s", diff)
				}
				if got.CompletedAt == nil || !got.CompletedAt.Equal(now) || got.ExpiresAt == nil || !got.ExpiresAt.Equal(now.Add(30*24*time.Hour)) {
					t.Errorf("unexpected completion and expiry: %v, %v", got.CompletedAt, got.ExpiresAt)
				}
			}
			if tc.phase == PostgresBackupPhaseFailed && (got.ErrorMessage == nil || *got.ErrorMessage != tc.wantErr) {
				t.Errorf("errorMessage = %v, want %q", got.ErrorMessage, tc.wantErr)
			}
		})
	}
}
