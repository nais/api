//go:build integration_test

package grpcuser_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/grpc/grpcuser"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserServer_Get(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	pool := getConnection(ctx, t, container, dsn, log)
	server := grpcuser.NewServer(pool)

	notFoundRequests := map[string]*protoapi.GetUserRequest{
		"Not found by ID":          {Id: uuid.New().String()},
		"Not found by email":       {Email: "email@example.com"},
		"Not found by external ID": {ExternalId: "id-that-does-not-exist"},
	}
	for name, req := range notFoundRequests {
		t.Run(name, func(t *testing.T) {
			resp, err := server.Get(ctx, req)
			if resp != nil {
				t.Error("expected response to be nil")
			}
			if s, ok := status.FromError(err); !ok || s.Code() != codes.NotFound {
				t.Errorf("expected status code %v, got %v", codes.NotFound, err)
			}
		})
	}

	t.Run("empty request", func(t *testing.T) {
		resp, err := server.Get(ctx, &protoapi.GetUserRequest{})
		if resp != nil {
			t.Fatalf("expected response to be nil and error to be not nil")
		}
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("expected status code %v, got %v", codes.InvalidArgument, err)
		}
	})

	t.Run("invalid uuid", func(t *testing.T) {
		resp, err := server.Get(ctx, &protoapi.GetUserRequest{Id: "invalid-uuid"})
		if resp != nil {
			t.Fatalf("expected response to be nil and error to be not nil")
		}
		if s, ok := status.FromError(err); !ok || s.Code() != codes.InvalidArgument {
			t.Errorf("expected status code %v, got %v", codes.InvalidArgument, err)
		}
	})

	userID := uuid.New()
	email := "user1@example.com"
	externalID := "123"
	stmt := "INSERT INTO users (id, name, email, external_id) VALUES ($1, 'User 1', $2, $3)"
	if _, err = pool.Exec(ctx, stmt, userID, email, externalID); err != nil {
		t.Fatalf("failed to insert user: %v", err)
	}

	successRequests := map[string]*protoapi.GetUserRequest{
		"By ID":          {Id: userID.String()},
		"By email":       {Email: email},
		"By external ID": {ExternalId: externalID},
	}
	for name, req := range successRequests {
		t.Run(name, func(t *testing.T) {
			resp, err := server.Get(ctx, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.User.Id != userID.String() {
				t.Errorf("expected user ID %v, got %v", userID, resp.User.Id)
			}
		})
	}
}

func TestUserServer_List(t *testing.T) {
	ctx := context.Background()
	log, _ := logrustest.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Run("no users", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)
		resp, err := grpcuser.NewServer(pool).List(ctx, &protoapi.ListUsersRequest{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 0 {
			t.Errorf("expected 0 users, got %d", len(resp.Nodes))
		}
	})

	t.Run("users", func(t *testing.T) {
		pool := getConnection(ctx, t, container, dsn, log)

		stmt := "INSERT INTO users (name, email, external_id) VALUES ('User 1', 'email1@example.com', '1')"
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		stmt = "INSERT INTO users (name, email, external_id) VALUES ('User 2', 'email2@example.com', '2')"
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		stmt = "INSERT INTO users (name, email, external_id) VALUES ('User 3', 'email3@example.com', '3')"
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		stmt = "INSERT INTO users (name, email, external_id) VALUES ('User 4', 'email4@example.com', '4')"
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		resp, err := grpcuser.NewServer(pool).List(ctx, &protoapi.ListUsersRequest{
			Limit:  2,
			Offset: 1,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(resp.Nodes) != 2 {
			t.Errorf("expected 2 users, got %d", len(resp.Nodes))
		}

		if resp.PageInfo.TotalCount != 4 {
			t.Errorf("expected 4 total users, got %d", resp.PageInfo.TotalCount)
		}

		if resp.Nodes[0].Name != "User 2" {
			t.Errorf("expected first user to be 'User 2', got %s", resp.Nodes[0].Name)
		}

		if resp.Nodes[1].Name != "User 3" {
			t.Errorf("expected second user to be 'User 3', got %s", resp.Nodes[1].Name)
		}
	})
}

func startPostgresql(ctx context.Context, t *testing.T, log logrus.FieldLogger) (container *postgres.PostgresContainer, dsn string, err error) {
	container, err = postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	defer testcontainers.CleanupContainer(t, container)

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
