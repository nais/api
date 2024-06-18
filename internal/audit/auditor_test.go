package audit_test

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/model"
	"github.com/testcontainers/testcontainers-go/wait"
	"io"
	"log"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAuditor(t *testing.T) {
	ctx := context.Background()
	container, connString, err := startPostgresql(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	db, closer, err := database.New(ctx, connString, logrus.New())
	if err != nil {
		t.Fatal(err)
	}

	actor := database.User{
		User: &gensql.User{
			ID:    uuid.New(),
			Email: "actor@example.com",
			Name:  "User Name",
		},
	}

	actorCtx := authz.ContextWithActor(ctx, actor, []*authz.Role{
		{
			RoleName: gensql.RoleNameAdmin,
			Authorizations: []roles.Authorization{
				roles.AuthorizationTeamsCreate,
			},
		},
	})

	auditor := audit.NewAuditor(db)

	t.Run("TeamMemberAdded", func(t *testing.T) {
		err := auditor.TeamMemberAdded(actorCtx, actor, "team", "target-user@example.com", model.TeamRoleOwner)
		if err != nil {
			t.Error(err)
		}
	})

	closer()
	err = container.Terminate(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func startPostgresql(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	lg := log.New(io.Discard, "", 0)
	if testing.Verbose() {
		lg = log.New(os.Stderr, "", log.LstdFlags)
	}

	container, err := postgres.RunContainer(ctx,
		testcontainers.WithLogger(lg),
		testcontainers.WithImage("docker.io/postgres:16-alpine"),
		postgres.WithDatabase("example"),
		postgres.WithUsername("example"),
		postgres.WithPassword("example"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2)),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	logr := logrus.New()
	logr.Out = io.Discard
	pool, err := database.NewPool(ctx, connStr, logr, true) // Migrate database before snapshotting
	if err != nil {
		return nil, "", fmt.Errorf("failed to create pool: %w", err)
	}

	pool.Close()
	if err := container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, connStr, nil
}
