//go:build integration_test

package database_test

import (
	"context"
	"database/sql"
	"errors"
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
		CREATE TABLE webhook_events (activity_log_entries_id uuid REFERENCES activity_log_entries(id) ON DELETE CASCADE);
		CREATE TABLE issues (issue_type text, resource_type text);
		INSERT INTO issues VALUES ('POSTGRES_BRANCH', 'POSTGRES_BRANCH'), ('POSTGRES', 'POSTGRES');
	`)
	if err != nil {
		t.Fatal(err)
	}
	type auditRecord struct {
		Actor, ResourceType, ResourceName, Team, Environment, Action string
		CreatedAt                                                    time.Time
		Data                                                         []byte
	}
	payload := []byte(`{"branch":"main","sourceBranch":"restored"}`)
	var retained int
	for _, tc := range []struct {
		resourceType, action string
		data                 []byte
		remove               bool
	}{
		{"POSTGRES", "BRANCH_CREATED", payload, true},
		{"POSTGRES", "BRANCH_ACTIVATED", payload, true},
		{"POSTGRES", "BRANCH_DELETED", payload, true},
		{"POSTGRES", "DELETED", nil, true},
		{"POSTGRES", "BRANCH_DELETED", []byte{}, true},
		{"POSTGRES", "BRANCH_CREATED", []byte("not json"), true},
		{"POSTGRES", "CREATED", payload, false},
		{"POSTGRES", "UPDATED", payload, false},
		{"POSTGRES", "DELETION_REQUESTED", nil, false},
		{"POSTGRES", "GRANT_ACCESS", payload, false},
		{"POSTGRES", "CREATE_PERSONAL_ACCESS", payload, false},
		{"POSTGRES", "GET_PERSONAL_ACCESS_CONNECTION", nil, false},
		{"OTHER", "BRANCH_CREATED", payload, false},
		{"OTHER", "DELETED", nil, false},
	} {
		t.Run(tc.resourceType+"/"+tc.action, func(t *testing.T) {
			id := uuid.New()
			want := auditRecord{
				Actor: "user@example.com", ResourceType: tc.resourceType, ResourceName: "orders", Team: "team", Environment: "dev", Action: tc.action,
				CreatedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), Data: tc.data,
			}
			_, err := db.ExecContext(ctx, `INSERT INTO activity_log_entries VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`, id, want.CreatedAt, want.Actor, want.ResourceType, want.ResourceName, want.Team, want.Environment, want.Action, want.Data)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.ExecContext(ctx, `INSERT INTO webhook_events VALUES ($1)`, id); err != nil {
				t.Fatal(err)
			}
			migration, err := os.ReadFile("migrations/0084_retire_postgres_branches.sql")
			if err != nil {
				t.Fatal(err)
			}
			// Cleanup is idempotent and does not parse the retired payload.
			for range 2 {
				if _, err := db.ExecContext(ctx, string(migration)); err != nil {
					t.Fatal(err)
				}
			}
			var got auditRecord
			err = db.QueryRowContext(ctx, `SELECT actor, resource_type, resource_name, team_slug, environment, action, created_at, data FROM activity_log_entries WHERE id = $1`, id).Scan(&got.Actor, &got.ResourceType, &got.ResourceName, &got.Team, &got.Environment, &got.Action, &got.CreatedAt, &got.Data)
			if tc.remove {
				if !errors.Is(err, sql.ErrNoRows) {
					t.Fatalf("retired entry still exists: %+v, %v", got, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if diff := cmp.Diff(want, got); diff != "" {
					t.Errorf("retained audit (-want +got):\n%s", diff)
				}
				retained++
			}
			var events int
			if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM webhook_events WHERE activity_log_entries_id = $1", id).Scan(&events); err != nil {
				t.Fatal(err)
			}
			if tc.remove && events != 0 || !tc.remove && events != 1 {
				t.Errorf("webhook events = %d, remove=%t", events, tc.remove)
			}
		})
	}
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"SELECT COUNT(*) FROM activity_log_entries", retained},
		{"SELECT COUNT(*) FROM issues WHERE issue_type = 'POSTGRES'", 1},
		{"SELECT COUNT(*) FROM issues WHERE issue_type = 'POSTGRES_BRANCH'", 0},
	} {
		var count int
		if err := db.QueryRowContext(ctx, tc.query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != tc.want {
			t.Errorf("%s = %d, want %d", tc.query, count, tc.want)
		}
	}
}
