//go:build integration_test

package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/serviceaccount"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestApiKeyAuthentication(t *testing.T) {
	ctx := context.Background()
	log, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	setup := func(t *testing.T) (context.Context, *pgxpool.Pool) {
		pool := getConnection(ctx, t, container, dsn, log)
		ctx = database.NewLoaderContext(ctx, pool)
		ctx = serviceaccount.NewLoaderContext(ctx, pool)
		ctx = authz.NewLoaderContext(ctx, pool)
		return ctx, pool
	}

	t.Run("No authorization header", func(t *testing.T) {
		ctx, _ := setup(t)

		apiKeyAuth := middleware.ApiKeyAuthentication()
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Fatal("expected nil actor")
			}
		})
		req := getRequest(ctx)
		apiKeyAuth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Unknown API key in header", func(t *testing.T) {
		ctx, _ := setup(t)

		apiKeyAuth := middleware.ApiKeyAuthentication()
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Fatal("expected nil actor")
			}
		})
		req := getRequest(ctx)
		req.Header.Set("Authorization", "Bearer unknown")
		apiKeyAuth(next).ServeHTTP(responseWriter, req)
	})
	t.Run("Valid API key", func(t *testing.T) {
		ctx, pool := setup(t)

		stmt := `
			INSERT INTO service_accounts (name, description) VALUES
			('sa1', 'description'),
			('sa2', 'description')`
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert service accounts: %v", err)
		}

		stmt = `
			INSERT INTO service_account_tokens (token, service_account_id, note) VALUES
		   ('key1', (SELECT id FROM service_accounts WHERE name = 'sa1'), 'note'),
		   ('key2', (SELECT id FROM service_accounts WHERE name = 'sa2'), 'note')`
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert service accounts: %v", err)
		}

		stmt = `
			INSERT INTO service_account_roles (role_name, service_account_id) VALUES
		   ('Deploy key viewer', (SELECT id FROM service_accounts WHERE name = 'sa1')),
		   ('Team creator', (SELECT id FROM service_accounts WHERE name = 'sa2'))`
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert service account roles: %v", err)
		}

		responseWriter := httptest.NewRecorder()
		next1 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor == nil {
				t.Fatal("expected actor")
			} else if !actor.User.IsServiceAccount() {
				t.Fatal("expected service account")
			} else if expected := "sa1"; actor.User.Identity() != expected {
				t.Fatalf("expected %q, got %q", expected, actor.User.Identity())
			} else if len(actor.Roles) != 1 {
				t.Fatal("expected one role")
			} else if expected := "Deploy key viewer"; string(actor.Roles[0].Name) != expected {
				t.Fatalf("expected role to be %q, got: %#v", expected, actor.Roles[0])
			}
		})
		next2 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor == nil {
				t.Fatal("expected actor")
			} else if !actor.User.IsServiceAccount() {
				t.Fatal("expected service account")
			} else if expected := "sa2"; actor.User.Identity() != expected {
				t.Fatalf("expected %q, got %q", expected, actor.User.Identity())
			} else if len(actor.Roles) != 1 {
				t.Fatal("expected one role")
			} else if expected := "Team creator"; string(actor.Roles[0].Name) != expected {
				t.Fatalf("expected role to be %q, got: %#v", expected, actor.Roles[0])
			}
		})

		req := getRequest(ctx)
		req.Header.Set("Authorization", "Bearer key1")
		middleware.ApiKeyAuthentication()(next1).ServeHTTP(responseWriter, req)

		req = getRequest(ctx)
		req.Header.Set("Authorization", "Bearer key2")
		middleware.ApiKeyAuthentication()(next2).ServeHTTP(responseWriter, req)
	})

	t.Run("Expired API key", func(t *testing.T) {
		ctx, pool := setup(t)

		stmt := `
			INSERT INTO service_accounts (name, description) VALUES
			('sa1', 'description')`
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert service accounts: %v", err)
		}

		stmt = `
			INSERT INTO service_account_tokens (token, service_account_id, expires_at, note) VALUES
		   ('key1', (SELECT id FROM service_accounts WHERE name = 'sa1'), '2021-01-01', 'note')`
		if _, err = pool.Exec(ctx, stmt); err != nil {
			t.Fatalf("failed to insert service accounts: %v", err)
		}

		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Fatal("expected nil actor")
			}
		})

		req := getRequest(ctx)
		req.Header.Set("Authorization", "Bearer key1")
		middleware.ApiKeyAuthentication()(next).ServeHTTP(responseWriter, req)
	})
}
