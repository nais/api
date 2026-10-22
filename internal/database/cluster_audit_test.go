//go:build integration_test

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/activitylog/activitylogsql"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestMoveClusterAuditsToActivityLog(t *testing.T) {
	ctx := context.Background()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("audit_test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := container.Terminate(context.Background()); err != nil {
			t.Error(err)
		}
	})
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	log := logrus.New()
	log.SetOutput(io.Discard)
	goose.SetLogger(log)
	goose.SetBaseFS(os.DirFS("."))
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpToContext(ctx, db, "migrations", 79); err != nil {
		t.Fatal(err)
	}

	id, uid := uuid.New(), uuid.New()
	longNamespaceID := uuid.New()
	longNamespace := "namespace-that-is-longer-than-thirty-characters"
	maxNamespace := strings.Repeat("a", 63)
	ts := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	_, err = db.ExecContext(ctx, `
		INSERT INTO k8s_events (id, uid, triggered_at, environment_name, involved_kind, involved_name, involved_namespace, reason, data)
		VALUES
			($1, $2, $3, 'dev', 'Deployment', 'app', 'my-team', 'Patch', '{"user":"user@example.com","method":"patch"}'),
			(gen_random_uuid(), $2, $3::timestamptz + INTERVAL '1 second', 'dev', 'Deployment', 'app', 'my-team', 'Delete', '{"user":"user@example.com"}'),
			(gen_random_uuid(), gen_random_uuid(), $3, 'dev', 'HorizontalPodAutoscaler', 'app', 'my-team', 'Patch', '{"user":"user@example.com"}'),
			($4, gen_random_uuid(), $3, 'dev', 'Deployment', 'app', $5, 'Patch', '{"user":"user@example.com"}'),
			(gen_random_uuid(), gen_random_uuid(), $3, 'dev', 'Deployment', 'app', $6, 'Patch', '{"user":"user@example.com"}'),
			(gen_random_uuid(), gen_random_uuid(), $3, 'dev', 'HorizontalPodAutoscaler', 'app', 'my-team', 'SuccessfulRescale', '{"newSize":"3","direction":"up"}'),
			(gen_random_uuid(), gen_random_uuid(), $3, 'dev', 'Pod', 'app-pod', 'my-team', 'Killing', '{"reason":"liveness_probe_failed","container":"app"}')
	`, id, uid, ts, longNamespaceID, longNamespace, maxNamespace)
	if err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		t.Fatal(err)
	}

	type auditEntry struct {
		Actor, Action, ResourceType, ResourceName, TeamSlug, Environment string
		CreatedAt                                                        time.Time
		SourceUID                                                        uuid.UUID
	}
	var got auditEntry
	var data string
	err = db.QueryRowContext(ctx, `
		SELECT actor, action, resource_type, resource_name, team_slug::text, environment,
			CONVERT_FROM(data, 'UTF8'), created_at, kubernetes_audit_uid
		FROM activity_log_entries WHERE id = $1
	`, id).Scan(&got.Actor, &got.Action, &got.ResourceType, &got.ResourceName, &got.TeamSlug, &got.Environment, &data, &got.CreatedAt, &got.SourceUID)
	if err != nil {
		t.Fatal(err)
	}
	want := auditEntry{
		Actor:        "user@example.com",
		Action:       "CLUSTER_AUDIT",
		ResourceType: "CLUSTER_AUDIT",
		ResourceName: "app",
		TeamSlug:     "my-team",
		Environment:  "dev",
		CreatedAt:    ts,
		SourceUID:    uid,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected migrated entry (-want +got):\n%s", diff)
	}
	var migratedNamespace string
	if err := db.QueryRowContext(ctx, "SELECT team_slug FROM activity_log_entries WHERE id = $1", longNamespaceID).Scan(&migratedNamespace); err != nil {
		t.Fatal(err)
	}
	if migratedNamespace != longNamespace {
		t.Errorf("got migrated namespace %q, want %q", migratedNamespace, longNamespace)
	}
	var payload map[string]string
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]string{"action": "Patch", "resourceKind": "Deployment"}, payload); diff != "" {
		t.Errorf("unexpected payload (-want +got):\n%s", diff)
	}
	assertCount := func(query string, want int) {
		t.Helper()
		var count int
		if err := db.QueryRowContext(ctx, query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != want {
			t.Errorf("query %q returned count %d, want %d", query, count, want)
		}
	}
	assertCount("SELECT COUNT(*) FROM activity_log_entries", 5)
	assertCount("SELECT COUNT(*) FROM activity_log_combined_view", 5)
	assertCount("SELECT COUNT(*) FROM activity_log_entries WHERE team_slug IS NULL", 0)
	assertCount("SELECT COUNT(*) FROM k8s_events", 2)
	assertCount("SELECT COUNT(*) FROM k8s_events WHERE data ->> 'user' IS NOT NULL", 0)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := activitylogsql.New(pool)
	params := activitylogsql.CreateClusterAuditParams{
		CreatedAt: pgtype.Timestamptz{Time: ts, Valid: true}, Actor: got.Actor,
		ResourceName: got.ResourceName, Namespace: got.TeamSlug, Data: []byte(data),
		EnvironmentName: &got.Environment, KubernetesAuditUid: uid,
	}
	// A redelivery after migration must not duplicate the historical entry.
	if err := q.CreateClusterAudit(ctx, params); err != nil {
		t.Fatal(err)
	}
	assertCount("SELECT COUNT(*) FROM activity_log_entries", 5)
	params.KubernetesAuditUid = uuid.New()
	params.Namespace = maxNamespace
	if err := q.CreateClusterAudit(ctx, params); err != nil {
		t.Fatal(err)
	}
	if err := q.CreateClusterAudit(ctx, params); err != nil {
		t.Fatal(err)
	}
	assertCount("SELECT COUNT(*) FROM activity_log_entries", 6)
	if err := db.QueryRowContext(ctx, "SELECT team_slug FROM activity_log_entries WHERE kubernetes_audit_uid = $1", params.KubernetesAuditUid).Scan(&migratedNamespace); err != nil {
		t.Fatal(err)
	}
	if migratedNamespace != maxNamespace {
		t.Errorf("got inserted namespace %q, want %q", migratedNamespace, maxNamespace)
	}
	params.CreatedAt.Time = ts.Add(2 * time.Second)
	if err := q.CreateClusterAudit(ctx, params); err != nil {
		t.Fatal(err)
	}
	assertCount("SELECT COUNT(*) FROM activity_log_entries", 7)
	// Keep the global team slug constraint unchanged.
	if _, err := db.ExecContext(ctx, "SELECT $1::slug", longNamespace); err == nil {
		t.Error("expected the team slug domain to reject a namespace longer than 30 characters")
	}
}
