//go:build integration_test

package grpcdeployment_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/deployment/deploymentsql"
	"github.com/nais/api/internal/grpc/grpcdeployment"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	"k8s.io/utils/ptr"
)

func TestDeploymentServer_Create(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("team not found", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		resp, err := grpcdeployment.NewServer(pool).CreateDeployment(ctx, &protoapi.CreateDeploymentRequest{
			TeamSlug: ptr.To("team-does-not-exist"),
		})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
			t.Errorf("expected status code %v, got %v", codes.NotFound, err)
		}
	})

	t.Run("missing environment", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		teamSlug := "my-team"
		stmt := `
				INSERT INTO teams (slug, purpose, slack_channel) VALUES
				($1, $2, $3)`
		if _, err = pool.Exec(ctx, stmt, teamSlug, "my-purpose", "#channel"); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcdeployment.NewServer(pool).CreateDeployment(ctx, &protoapi.CreateDeploymentRequest{
			TeamSlug: &teamSlug,
		})
		if resp != nil {
			t.Error("expected response to be nil")
		}

		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("expected status code %v, got %v", codes.InvalidArgument, err)
		} else if contains, e := "environment is required", err.Error(); !strings.Contains(e, contains) {
			t.Errorf("expected error message to contain %q, got %q", contains, e)
		}
	})

	t.Run("only required", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		environmentName, teamSlug := "prod", "my-team"
		stmt := `
			INSERT INTO teams (slug, purpose, slack_channel) VALUES
			($1, $2, $3)`
		if _, err = pool.Exec(ctx, stmt, teamSlug, "my-purpose", "#channel"); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcdeployment.NewServer(pool).CreateDeployment(ctx, &protoapi.CreateDeploymentRequest{
			TeamSlug:        &teamSlug,
			EnvironmentName: &environmentName,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		} else if resp == nil {
			t.Fatalf("expected response to be non-nil")
		}

		var d deploymentsql.Deployment
		stmt = "SELECT * FROM deployments WHERE id = $1::UUID"
		if err := pool.QueryRow(ctx, stmt, resp.Id).Scan(&d.ID, &d.ExternalID, &d.CreatedAt, &d.TeamSlug, &d.Repository, &d.CommitSha, &d.DeployerUsername, &d.TriggerUrl, &d.EnvironmentName); err != nil {
			t.Fatalf("failed to get deployment: %v", err)
		}

		if d.ID.String() != *resp.Id {
			t.Errorf("expected deployment ID to be %q, got %q", *resp.Id, d.ID.String())
		}

		if d.ExternalID != nil {
			t.Errorf("expected external ID to be nil, got %q", *d.ExternalID)
		}

		if !d.CreatedAt.Time.Before(time.Now()) || !d.CreatedAt.Time.After(time.Now().Add(-2*time.Second)) {
			t.Errorf("expected created at to be before now and at most two seconds ago, got %v", d.CreatedAt.Time)
		}

		if d.TeamSlug.String() != teamSlug {
			t.Errorf("expected team slug to be %q, got %q", teamSlug, d.TeamSlug)
		}

		if d.Repository != nil {
			t.Errorf("expected repository to be nil, got %q", *d.Repository)
		}

		if d.CommitSha != nil {
			t.Errorf("expected commit sha to be nil, got %q", *d.CommitSha)
		}

		if d.DeployerUsername != nil {
			t.Errorf("expected deployer username to be nil, got %q", *d.DeployerUsername)
		}

		if d.TriggerUrl != nil {
			t.Errorf("expected trigger URL to be nil, got %q", *d.TriggerUrl)
		}

		if d.EnvironmentName != environmentName {
			t.Errorf("expected environment name to be %q, got %q", environmentName, d.EnvironmentName)
		}
	})

	t.Run("all fields set", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		environmentName := "prod"
		teamSlug := "my-team"
		externalID := "ext-id"
		createdAt := time.Date(2025, 1, 1, 1, 1, 1, 0, time.UTC)
		repository := "repo"
		commitSha := "sha"
		deployerUsername := "deployer"
		triggerUrl := "url"

		stmt := `
			INSERT INTO teams (slug, purpose, slack_channel) VALUES
			($1, $2, $3)`
		if _, err = pool.Exec(ctx, stmt, teamSlug, "my-purpose", "#channel"); err != nil {
			t.Fatalf("failed to insert team: %v", err)
		}

		resp, err := grpcdeployment.NewServer(pool).CreateDeployment(ctx, &protoapi.CreateDeploymentRequest{
			CreatedAt:        timestamppb.New(createdAt),
			ExternalId:       &externalID,
			TeamSlug:         &teamSlug,
			Repository:       &repository,
			EnvironmentName:  &environmentName,
			CommitSha:        &commitSha,
			DeployerUsername: &deployerUsername,
			TriggerUrl:       &triggerUrl,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		} else if resp == nil {
			t.Fatalf("expected response to be non-nil")
		}

		var d deploymentsql.Deployment
		stmt = "SELECT * FROM deployments WHERE id = $1::UUID"
		if err := pool.QueryRow(ctx, stmt, resp.Id).Scan(&d.ID, &d.ExternalID, &d.CreatedAt, &d.TeamSlug, &d.Repository, &d.CommitSha, &d.DeployerUsername, &d.TriggerUrl, &d.EnvironmentName); err != nil {
			t.Fatalf("failed to get deployment: %v", err)
		}

		if d.ID.String() != *resp.Id {
			t.Errorf("expected deployment ID to be %q, got %q", *resp.Id, d.ID.String())
		}

		if *d.ExternalID != externalID {
			t.Errorf("expected external ID to be nil, got %q", *d.ExternalID)
		}

		if !d.CreatedAt.Time.Equal(createdAt) {
			t.Errorf("expected created at to be %v, got %v", createdAt.UTC(), d.CreatedAt.Time.UTC())
		}

		if d.TeamSlug.String() != teamSlug {
			t.Errorf("expected team slug to be %q, got %q", teamSlug, d.TeamSlug)
		}

		if *d.Repository != repository {
			t.Errorf("expected repository to be nil, got %q", *d.Repository)
		}

		if *d.CommitSha != commitSha {
			t.Errorf("expected commit sha to be nil, got %q", *d.CommitSha)
		}

		if *d.DeployerUsername != deployerUsername {
			t.Errorf("expected deployer username to be nil, got %q", *d.DeployerUsername)
		}

		if *d.TriggerUrl != triggerUrl {
			t.Errorf("expected trigger URL to be nil, got %q", *d.TriggerUrl)
		}

		if d.EnvironmentName != environmentName {
			t.Errorf("expected environment name to be %q, got %q", environmentName, d.EnvironmentName)
		}
	})
}

func startPostgresql(ctx context.Context, log logrus.FieldLogger) (container *postgres.PostgresContainer, dsn string, err error) {
	container, err = postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	pool, err := database.NewPool(ctx, dsn, log, true)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create pool: %w", err)
	}
	pool.Close()

	if err := container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, dsn, nil
}

func getConnection(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log logrus.FieldLogger) *pgxpool.Pool {
	pool, _ := database.NewPool(ctx, dsn, log, false)

	t.Cleanup(func() {
		pool.Close()
		if err := container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	return pool
}
