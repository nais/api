//go:build integration_test

package deployment_test

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/deployment/deploymentsql"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestLatestDeploymentTimestampForWorkload(t *testing.T) {
	ctx := t.Context()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	log, _ := logrustest.NewNullLogger()
	pool, err := database.NewPool(ctx, dsn, log, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	older := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newer := older.Add(time.Hour)
	for _, workload := range []struct {
		kind         string
		resourceType string
	}{
		{kind: "Application", resourceType: "APP"},
		{kind: "Naisjob", resourceType: "JOB"},
	} {
		t.Run(workload.kind, func(t *testing.T) {
			for _, test := range []struct {
				name           string
				deploymentTime time.Time
				activityTime   time.Time
				action         string
				want           time.Time
			}{
				{name: "no history"},
				{name: "deployment only", deploymentTime: older, want: older},
				{name: "created only", activityTime: newer, action: "CREATED", want: newer},
				{name: "updated only", activityTime: newer, action: "UPDATED", want: newer},
				{name: "creation newer than deployment", deploymentTime: older, activityTime: newer, action: "CREATED", want: newer},
				{name: "update newer than deployment", deploymentTime: older, activityTime: newer, action: "UPDATED", want: newer},
				{name: "deployment newer than update", deploymentTime: newer, activityTime: older, action: "UPDATED", want: newer},
				{name: "unrelated action", deploymentTime: older, activityTime: newer, action: "DELETED", want: older},
			} {
				t.Run(test.name, func(t *testing.T) {
					tx, err := pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					defer func() {
						if err := tx.Rollback(ctx); err != nil {
							t.Error(err)
						}
					}()
					if !test.deploymentTime.IsZero() {
						_, err := tx.Exec(ctx, `
							WITH deployment AS (
								INSERT INTO deployments (created_at, team_slug, environment_name)
								VALUES ($1, 'my-team', 'dev')
								RETURNING id
							)
							INSERT INTO deployment_k8s_resources (deployment_id, "group", version, kind, name, namespace)
							SELECT id, 'nais.io', 'v1alpha1', $2, 'my-workload', 'my-team' FROM deployment
						`, test.deploymentTime, workload.kind)
						if err != nil {
							t.Fatal(err)
						}
					}
					if !test.activityTime.IsZero() {
						_, err := tx.Exec(ctx, `
							INSERT INTO activity_log_entries (created_at, actor, action, resource_type, resource_name, team_slug, environment)
							VALUES ($1, 'user@example.com', $2, $3, 'my-workload', 'my-team', 'dev')
						`, test.activityTime, test.action, workload.resourceType)
						if err != nil {
							t.Fatal(err)
						}
					}
					_, err = tx.Exec(ctx, `
						INSERT INTO activity_log_entries (created_at, actor, action, resource_type, resource_name, team_slug, environment)
						VALUES
							($1, 'user@example.com', 'UPDATED', $2, 'other-workload', 'my-team', 'dev'),
							($1, 'user@example.com', 'UPDATED', $2, 'my-workload', 'other-team', 'dev'),
							($1, 'user@example.com', 'UPDATED', $2, 'my-workload', 'my-team', 'prod'),
							($1, 'user@example.com', 'UPDATED', 'ConfigMap', 'my-workload', 'my-team', 'dev'),
							($1, 'user@example.com', 'CREATED', CASE WHEN $2 = 'APP' THEN 'JOB' ELSE 'APP' END, 'my-workload', 'my-team', 'dev')
					`, newer.Add(time.Hour), workload.resourceType)
					if err != nil {
						t.Fatal(err)
					}
					got, err := deploymentsql.New(tx).LatestDeploymentTimestampForWorkload(ctx, deploymentsql.LatestDeploymentTimestampForWorkloadParams{
						WorkloadName:    "my-workload",
						WorkloadKind:    workload.kind,
						EnvironmentName: "dev",
						TeamSlug:        "my-team",
					})
					if test.want.IsZero() {
						if !errors.Is(err, pgx.ErrNoRows) {
							t.Fatalf("expected no rows, got timestamp %v and error %v", got, err)
						}
						return
					}
					if err != nil {
						t.Fatal(err)
					}
					if !got.Valid || !got.Time.Equal(test.want) {
						t.Errorf("got %v, want %v", got, test.want)
					}
				})
			}
		})
	}
}
