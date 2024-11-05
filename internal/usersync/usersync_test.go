//go:build integration_test
// +build integration_test

package usersync_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/test"
	"github.com/nais/api/internal/usersync"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/role/rolesql"
	"github.com/nais/api/internal/v1/user"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/vikstrous/dataloadgen"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

const (
	domain           = "example.com"
	adminGroupPrefix = "nais-admins"
)

func TestSync(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, connStr, cleanup, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}
	defer cleanup()

	pool, cleanup, err := newDB(ctx, container, connStr, log)
	if err != nil {
		t.Fatalf("failed to create database: %v", err)
	}
	defer cleanup()

	ctx = databasev1.NewLoaderContext(ctx, pool)
	ctx = user.NewLoaderContext(ctx, pool, []dataloadgen.Option{})
	ctx = role.NewLoaderContext(ctx, pool, []dataloadgen.Option{})

	t.Run("No local users, no remote users", func(t *testing.T) {
		correlationID := uuid.New()

		httpClient := test.NewTestHttpClient(
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"users":[]}`)
			},
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"members":[]}`)
			},
		)

		svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			t.Fatal(err)
		}

		err = usersync.
			New(pool, adminGroupPrefix, domain, svc, log).
			Sync(ctx, correlationID)
		if err != nil {
			t.Fatal(err)
		}

		p, _ := pagination.ParsePage(nil, nil, nil, nil)
		if users, err := user.List(ctx, p, nil); err != nil {
			t.Fatal(err)
		} else if total := len(users.Nodes()); total != 0 {
			t.Fatalf("expected 0 users, got %d", total)
		}
	})

	t.Run("Local users, no remote users", func(t *testing.T) {
		correlationID := uuid.New()

		user1, err := user.Create(ctx, "User 1", "user1@example.com", "123")
		if err != nil {
			t.Fatal(err)
		}

		user2, err := user.Create(ctx, "User 2", "user2@example.com", "456")
		if err != nil {
			t.Fatal(err)
		}

		if err := role.AssignGlobalRoleToUser(ctx, user1.UUID, rolesql.RoleNameTeamcreator); err != nil {
			t.Fatal(err)
		}

		if err := role.AssignGlobalRoleToUser(ctx, user2.UUID, rolesql.RoleNameAdmin); err != nil {
			t.Fatal(err)
		}

		httpClient := test.NewTestHttpClient(
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"users":[]}`)
			},
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"members":[]}`)
			},
		)
		svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			t.Fatal(err)
		}

		err = usersync.
			New(pool, adminGroupPrefix, domain, svc, log).
			Sync(ctx, correlationID)
		if err != nil {
			t.Fatal(err)
		}

		p, _ := pagination.ParsePage(nil, nil, nil, nil)
		if users, err := user.List(ctx, p, nil); err != nil {
			t.Fatal(err)
		} else if total := len(users.Nodes()); total != 0 {
			t.Fatalf("expected 0 users, got %d", total)
		}
	})

	t.Run("Create, update and delete users", func(t *testing.T) {
		correlationID := uuid.New()

		userWithIncorrectName, err := user.Create(ctx, "Incorrect Name", "user1@example.com", "1")
		if err != nil {
			t.Fatal(err)
		}

		userWithIncorrectEmail, err := user.Create(ctx, "Some Name", "incorrect@example.com", "2")
		if err != nil {
			t.Fatal(err)
		}

		userThatWillBeDeleted, err := user.Create(ctx, "Delete Me", "delete-me@example.com", "3")
		if err != nil {
			t.Fatal(err)
		}

		userThatShouldLoseAdminRole, err := user.Create(ctx, "Should Lose Admin", "should-lose-admin@example.com", "4")
		if err != nil {
			t.Fatal(err)
		}

		if err := role.AssignGlobalRoleToUser(ctx, userThatShouldLoseAdminRole.UUID, rolesql.RoleNameAdmin); err != nil {
			t.Fatal(err)
		}

		httpClient := test.NewTestHttpClient(
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"users":[`+
					`{"id": "1", "primaryEmail":"user1@example.com","name":{"fullName":"Correct Name"}},`+ // Will update name of local user
					`{"id": "2", "primaryEmail":"user2@example.com","name":{"fullName":"Some Name"}},`+ // Will update email of local user
					`{"id": "4", "primaryEmail":"should-lose-admin@example.com","name":{"fullName":"Should Lose Admin"}},`+ // Will lose admin role
					`{"id": "5", "primaryEmail":"create-me@example.com","name":{"fullName":"Create Me"}}]}`) // Will be created
			},
			func(req *http.Request) *http.Response {
				return test.Response("200 OK", `{"members":[`+
					`{"id": "2", "email":"user2@example.com", "status": "ACTIVE", "type": "USER"},`+ // Will be granted admin role
					`{"Id": "6", "email":"some-group@example.com", "type": "GROUP"},`+ // Group type, will be ignored
					`{"Id": "7", "email":"unknown-admin@example.com", "status": "ACTIVE", "type": "USER"},`+ // Unknown user, will be logged
					`{"Id": "8", "email":"inactive-user@example.com", "status":"SUSPENDED", "type": "USER"}]}`) // Invalid status, will be ignored
			},
		)
		svc, err := admindirectoryv1.NewService(ctx, option.WithHTTPClient(httpClient))
		if err != nil {
			t.Fatal(err)
		}

		err = usersync.
			New(pool, adminGroupPrefix, domain, svc, log).
			Sync(ctx, correlationID)
		if err != nil {
			t.Fatal(err)
		}

		p, _ := pagination.ParsePage(nil, nil, nil, nil)
		if users, err := user.List(ctx, p, nil); err != nil {
			t.Fatal(err)
		} else if total := len(users.Nodes()); total != 4 {
			t.Fatalf("expected 3 users, got %d", total)
		}

		if u, err := user.Get(ctx, userWithIncorrectName.UUID); err != nil {
			t.Fatal(err)
		} else if correctName := "Correct Name"; u.Name != correctName {
			t.Fatalf("expected name to be %q, got %q", correctName, u.Name)
		}

		if u, err := user.Get(ctx, userWithIncorrectEmail.UUID); err != nil {
			t.Fatal(err)
		} else if correctEmail := "user2@example.com"; u.Email != correctEmail {
			t.Fatalf("expected email to be %q, got %q", correctEmail, u.Email)
		}

		if u, err := user.Get(ctx, userThatWillBeDeleted.UUID); err == nil {
			t.Fatalf("expected user to be deleted, got %v", u)
		}

		if u, err := user.GetByEmail(ctx, "create-me@example.com"); err != nil {
			t.Fatal(err)
		} else if correctName := "Create Me"; u.Name != correctName {
			t.Fatalf("expected name to be %q, got %q", correctName, u.Name)
		}

		roles, err := role.ForUser(ctx, userThatShouldLoseAdminRole.UUID)
		if err != nil {
			t.Fatal(err)
		}

		for _, r := range roles {
			if r.Name == rolesql.RoleNameAdmin {
				t.Fatalf("expected user to lose admin role, but still has it")
			}
		}

		roles, err = role.ForUser(ctx, userWithIncorrectEmail.UUID)
		if err != nil {
			t.Fatal(err)
		}

		foundAdminRole := false
		for _, r := range roles {
			if r.Name == rolesql.RoleNameAdmin {
				foundAdminRole = true
				break
			}
		}
		if !foundAdminRole {
			t.Fatalf("expected user to be granted admin role, but doesn't have it")
		}
	})
}

func startPostgresql(ctx context.Context, log logrus.FieldLogger) (*postgres.PostgresContainer, string, func(), error) {
	container, err := postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("usersync"),
		postgres.WithUsername("usersync"),
		postgres.WithPassword("usersync"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	pool, err := databasev1.NewPool(ctx, connStr, log, true)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create pool: %w", err)
	}
	pool.Close()

	if err := container.Snapshot(ctx, postgres.WithSnapshotName("migrated")); err != nil {
		return nil, "", nil, fmt.Errorf("failed to snapshot: %w", err)
	}

	cleanup := func() {
		_ = testcontainers.TerminateContainer(container)
	}
	return container, connStr, cleanup, nil
}

func newDB(ctx context.Context, postgresContainer *postgres.PostgresContainer, connectionString string, log logrus.FieldLogger) (*pgxpool.Pool, func(), error) {
	pool, err := databasev1.NewPool(ctx, connectionString, log, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	cleanup := func() {
		pool.Close()
		_ = postgresContainer.Restore(ctx)
	}

	return pool, cleanup, nil
}
