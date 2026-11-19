//go:build integration_test

package database_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestRetirePostgresBranches(t *testing.T) {
	ctx := t.Context()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine", postgres.WithDatabase("history"), postgres.WithUsername("test"), postgres.WithPassword("test"), postgres.BasicWaitStrategies())
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
	_, err = db.ExecContext(ctx, `
		CREATE TABLE activity_log_entries (id uuid PRIMARY KEY, created_at timestamptz NOT NULL, actor text NOT NULL, resource_type text NOT NULL, resource_name text NOT NULL, team_slug text, environment text, action text NOT NULL, data bytea);
		CREATE TABLE issues (issue_type text, resource_type text);
		INSERT INTO issues VALUES ('POSTGRES_BRANCH', 'POSTGRES_BRANCH'), ('POSTGRES', 'POSTGRES');
	`)
	if err != nil {
		t.Fatal(err)
	}
	timestamp := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	type record struct {
		id      uuid.UUID
		action  string
		payload *string
	}
	payload := `{"branch": "main", "sourceBranch": "restored", "targetTime": "2026-01-01T00:00:00Z"}`
	empty := ""
	records := []record{
		{uuid.New(), "BRANCH_CREATED", &payload},
		{uuid.New(), "BRANCH_ACTIVATED", &payload},
		{uuid.New(), "BRANCH_DELETED", &payload},
		{uuid.New(), "DELETED", nil},
		{uuid.New(), "BRANCH_DELETED", &empty},
		{uuid.New(), "CREATED", &payload},
		{uuid.New(), "DELETION_REQUESTED", nil},
	}
	for _, row := range records {
		var data []byte
		if row.payload != nil {
			data = []byte(*row.payload)
		}
		_, err := db.ExecContext(ctx, `INSERT INTO activity_log_entries VALUES ($1, $2, 'user@example.com', 'POSTGRES', 'orders', 'team', 'dev', $3, $4)`, row.id, timestamp, row.action, data)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO activity_log_entries VALUES ($1, $2, 'other', 'OTHER', 'orders', NULL, NULL, 'BRANCH_CREATED', NULL)`, uuid.New(), timestamp); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("migrations/0084_retire_postgres_branches.sql")
	if err != nil {
		t.Fatal(err)
	}
	// Re-execution must not nest or overwrite the preserved history.
	for range 2 {
		if _, err := db.ExecContext(ctx, string(migration)); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range records {
		var actor, resourceType, name, team, env, action string
		var created time.Time
		var data []byte
		err := db.QueryRowContext(ctx, `SELECT actor, resource_type, resource_name, team_slug, environment, action, created_at, data FROM activity_log_entries WHERE id = $1`, row.id).Scan(&actor, &resourceType, &name, &team, &env, &action, &created, &data)
		if err != nil {
			t.Fatal(err)
		}
		if actor != "user@example.com" || resourceType != "POSTGRES" || name != "orders" || team != "team" || env != "dev" || !created.Equal(timestamp) {
			t.Error("audit identity or scope changed")
		}
		if row.action == "CREATED" || row.action == "DELETION_REQUESTED" {
			if action != row.action {
				t.Errorf("unrelated action changed to %q", action)
			}
			if row.payload != nil && string(data) != *row.payload {
				t.Error("unrelated payload changed")
			}
			continue
		}
		if action != "HISTORICAL" {
			t.Fatalf("action = %q", action)
		}
		var got struct {
			OriginalAction string  `json:"originalAction"`
			OriginalData   *string `json:"originalData"`
		}
		if err := json.Unmarshal(data, &got); err != nil {
			t.Fatal(err)
		}
		if got.OriginalAction != row.action {
			t.Errorf("original action = %q, want %q", got.OriginalAction, row.action)
		}
		if diff := cmp.Diff(row.payload, got.OriginalData); diff != "" {
			t.Errorf("original payload (-want +got):\n%s", diff)
		}
	}
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM activity_log_entries").Scan(&count); err != nil || count != len(records)+1 {
		t.Fatalf("audit count = %d, %v", count, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM issues WHERE issue_type = 'POSTGRES'").Scan(&count); err != nil || count != 1 {
		t.Fatalf("current issues = %d, %v", count, err)
	}
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM issues WHERE issue_type = 'POSTGRES_BRANCH'").Scan(&count); err != nil || count != 0 {
		t.Fatalf("obsolete issues = %d, %v", count, err)
	}
}
