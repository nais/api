package postgres

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
	naisv1 "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPostgresBackupConnectionFiltersAndPagesIndependentlyOfSource(t *testing.T) {
	var items []unstructured.Unstructured
	for i, names := range [][2]string{{"first", "deleted-db"}, {"unrelated", "another-db"}, {"newest", "deleted-db"}} {
		backup := naisv1.PostgresBackup{
			ObjectMeta: metav1.ObjectMeta{Name: names[0], Namespace: "team", CreationTimestamp: metav1.NewTime(time.Unix(int64(i), 0))},
			Spec:       naisv1.PostgresBackupSpec{Postgres: names[1]},
		}
		obj, err := kubernetes.ToUnstructured(&backup)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, *obj)
	}
	page, err := pagination.ParsePage(new(1), nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := postgresBackupConnection(items, "dev", page, &PostgresBackupFilter{SourcePostgres: new("deleted-db")})
	if err != nil {
		t.Fatal(err)
	}
	if nodes := conn.Nodes(); len(nodes) != 1 || nodes[0].Name != "newest" || conn.PageInfo.TotalCount != 2 || !conn.PageInfo.HasNextPage {
		t.Fatalf("unexpected filtered page: %+v", conn)
	}
}

func TestPostgresBackupPublicStatus(t *testing.T) {
	now := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name   string
		status *naisv1.PostgresBackupStatus
		phase  PostgresBackupPhase
	}{
		{"new request", nil, PostgresBackupPhasePreparing},
		{"copying", &naisv1.PostgresBackupStatus{Phase: naisv1.PostgresBackupPhaseRunning}, PostgresBackupPhaseRunning},
		{"completed with a size above 32 bits", &naisv1.PostgresBackupStatus{
			Phase:       naisv1.PostgresBackupPhaseCompleted,
			CompletedAt: new(metav1.NewTime(now)), ExpiresAt: new(metav1.NewTime(now.Add(30 * 24 * time.Hour))), SizeBytes: new(int64(5 << 30)),
		}, PostgresBackupPhaseCompleted},
		{"failed", &naisv1.PostgresBackupStatus{Phase: naisv1.PostgresBackupPhaseFailed, ErrorMessage: "internal bucket name"}, PostgresBackupPhaseFailed},
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
				if got.CompletedAt == nil || !got.CompletedAt.Equal(now) || got.ExpiresAt == nil || !got.ExpiresAt.Equal(now.Add(30*24*time.Hour)) {
					t.Errorf("unexpected completion and expiry: %v, %v", got.CompletedAt, got.ExpiresAt)
				}
			}
			if tc.phase == PostgresBackupPhaseFailed && (got.ErrorMessage == nil || *got.ErrorMessage == tc.status.ErrorMessage) {
				t.Error("failed backups should report an error without exposing internal details")
			}
		})
	}
}
