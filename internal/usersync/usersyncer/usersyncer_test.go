//go:build integration_test

package usersyncer_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/test"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/usersync/usersyncer"
	"github.com/nais/api/internal/usersync/usersyncsql"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	admindirectoryv1 "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/option"
)

const (
	domain           = "example.com"
	adminGroupPrefix = "console-admins"
)

func TestSync(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	setup := func(t *testing.T) (context.Context, *pgxpool.Pool) {
		pool := getConnection(ctx, t, container, dsn, log)
		ctx = database.NewLoaderContext(ctx, pool)
		ctx = user.NewLoaderContext(ctx, pool)
		return ctx, pool
	}
	t.Run("No local users, no remote users", func(t *testing.T) {
		ctx, pool := setup(t)

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

		// TODO: Add tests for Zitadel API operations
		err = usersyncer.
			New(pool, adminGroupPrefix, domain, nil, svc, log).
			Sync(ctx)
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
		ctx, pool := setup(t)
		querier := usersyncsql.New(pool)

		user1, err := querier.Create(ctx, usersyncsql.CreateParams{
			Name:       "User 1",
			Email:      "user1@example.com",
			ExternalID: "123",
		})
		if err != nil {
			t.Fatal(err)
		}

		user2, err := querier.Create(ctx, usersyncsql.CreateParams{
			Name:       "User 2",
			Email:      "user2@example.com",
			ExternalID: "456",
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := querier.AssignGlobalRole(ctx, usersyncsql.AssignGlobalRoleParams{
			UserID:   user1.ID,
			RoleName: "Team creator",
		}); err != nil {
			t.Fatal(err)
		}

		if err := querier.AssignGlobalAdmin(ctx, user2.ID); err != nil {
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

		err = usersyncer.
			New(pool, adminGroupPrefix, domain, nil, svc, log).
			Sync(ctx)
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
		ctx, pool := setup(t)
		querier := usersyncsql.New(pool)

		userWithIncorrectName, err := querier.Create(ctx, usersyncsql.CreateParams{
			Name:       "Incorrect Name",
			Email:      "user1@example.com",
			ExternalID: "1",
		})
		if err != nil {
			t.Fatal(err)
		}

		userWithIncorrectEmail, err := querier.Create(ctx, usersyncsql.CreateParams{
			Name:       "Some Name",
			Email:      "incorrect@example.com",
			ExternalID: "2",
		})
		if err != nil {
			t.Fatal(err)
		}

		userThatWillBeDeleted, err := querier.Create(ctx, usersyncsql.CreateParams{
			Name:       "Delete Me",
			Email:      "delete-me@example.com",
			ExternalID: "3",
		})
		if err != nil {
			t.Fatal(err)
		}

		userThatShouldLoseAdminRole, err := querier.Create(ctx, usersyncsql.CreateParams{
			Name:       "Should Lose Admin",
			Email:      "should-lose-admin@example.com",
			ExternalID: "4",
		})
		if err != nil {
			t.Fatal(err)
		}

		if err := querier.AssignGlobalAdmin(ctx, userThatShouldLoseAdminRole.ID); err != nil {
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

		err = usersyncer.
			New(pool, adminGroupPrefix, domain, nil, svc, log).
			Sync(ctx)
		if err != nil {
			t.Fatal(err)
		}

		p, _ := pagination.ParsePage(nil, nil, nil, nil)
		if users, err := user.List(ctx, p, nil); err != nil {
			t.Fatal(err)
		} else if total := len(users.Nodes()); total != 4 {
			t.Fatalf("expected 3 users, got %d", total)
		}

		if u, err := user.Get(ctx, userWithIncorrectName.ID); err != nil {
			t.Fatal(err)
		} else if correctName := "Correct Name"; u.Name != correctName {
			t.Fatalf("expected name to be %q, got %q", correctName, u.Name)
		}

		if u, err := user.Get(ctx, userWithIncorrectEmail.ID); err != nil {
			t.Fatal(err)
		} else if correctEmail := "user2@example.com"; u.Email != correctEmail {
			t.Fatalf("expected email to be %q, got %q", correctEmail, u.Email)
		}

		if u, err := user.Get(ctx, userThatWillBeDeleted.ID); err == nil {
			t.Fatalf("expected user to be deleted, got %v", u)
		}

		if u, err := user.GetByEmail(ctx, "create-me@example.com"); err != nil {
			t.Fatal(err)
		} else if correctName := "Create Me"; u.Name != correctName {
			t.Fatalf("expected name to be %q, got %q", correctName, u.Name)
		}

		updatedUserThatShouldLoseAdmin, err := user.Get(ctx, userThatShouldLoseAdminRole.ID)
		if err != nil {
			t.Fatal(err)
		}
		if updatedUserThatShouldLoseAdmin.Admin {
			t.Fatalf("expected user to lose admin role, but still has it")
		}

		updatedUserWithIncorrectEmail, err := user.Get(ctx, userWithIncorrectEmail.ID)
		if err != nil {
			t.Fatal(err)
		}

		if !updatedUserWithIncorrectEmail.Admin {
			t.Fatalf("expected user to be granted admin role, but doesn't have it")
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
