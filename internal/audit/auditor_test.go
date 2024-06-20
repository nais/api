package audit_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/roles"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestAuditor(t *testing.T) {
	ctx := context.Background()
	container, connString, err := startPostgres(context.Background())
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

	team := "team"

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
		if err := auditor.TeamMemberAdded(actorCtx, actor, slug.Slug(team), "target-user@example.com", model.TeamRoleOwner); err != nil {
			t.Error(err)
		}

		db.GetAuditEventsForTeam(ctx, slug.Slug(team), database.Page{Limit: 10, Offset: 0})

		events, _, err := db.GetAuditEventsForTeam(ctx, slug.Slug(team), database.Page{Limit: 10, Offset: 0})
		if err != nil {
			t.Error(err)
		}

		if len(events) != 1 {
			t.Errorf("got %d, want 1", len(events))
		}

		it := events[0]

		if it.Actor != actor.Identity() {
			t.Errorf("got %s, want %s", it.Actor, actor.Identity())
		}

		if it.ResourceType != string(model.AuditEventResourceTypeTeamMember) {
			t.Errorf("got %s, want %s", it.ResourceType, model.AuditEventResourceTypeTeamMember)
		}

		if it.TeamSlug.String() != team {
			t.Errorf("got %s, want %s", it.TeamSlug, team)
		}

		if it.ResourceName != team {
			t.Errorf("got %s, want %s", it.ResourceName, team)
		}

		if it.Data == nil {
			t.Error("expected data")
		}

		var d model.AuditEventMemberAddedData
		if err := json.Unmarshal(it.Data, &d); err != nil {
			t.Error(err)
		}

		if d.Role != model.TeamRoleOwner {
			t.Errorf("got %s, want %s", d.Role, model.TeamRoleOwner)
		}
	})

	closer()
	err = container.Terminate(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func startPostgres(ctx context.Context) (*postgres.PostgresContainer, string, error) {
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
