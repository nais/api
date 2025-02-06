//go:build integration_test

package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/session"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/mock"
)

func TestOauth2Authentication(t *testing.T) {
	ctx := context.Background()
	log, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, log)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	setup := func(t *testing.T) (context.Context, *pgxpool.Pool) {
		pool := getConnection(ctx, t, container, dsn, log)
		ctx = database.NewLoaderContext(ctx, pool)
		ctx = session.NewLoaderContext(ctx, pool)
		ctx = user.NewLoaderContext(ctx, pool)
		ctx = authz.NewLoaderContext(ctx, pool)
		return ctx, pool
	}

	t.Run("No cookie in request", func(t *testing.T) {
		ctx, _ := setup(t)

		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Errorf("unexpected actor: %v", actor)
			}
		})
		req := getRequest(ctx)
		oauth2Auth := middleware.Oauth2Authentication(authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Invalid cookie value", func(t *testing.T) {
		ctx, _ := setup(t)

		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Errorf("unexpected actor: %v", actor)
			}
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: "unknown-session-key",
		})
		oauth2Auth := middleware.Oauth2Authentication(authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie, no session in store", func(t *testing.T) {
		ctx, _ := setup(t)

		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Errorf("unexpected actor: %v", actor)
			}
		})
		req := getRequest(ctx)
		sessionID := uuid.New()
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})
		oauth2Auth := middleware.Oauth2Authentication(authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie with matching session", func(t *testing.T) {
		ctx, pool := setup(t)

		userID := uuid.New()
		stmt := "INSERT INTO users (id, name, email, external_id) VALUES ($1, 'User 1', 'user1@example.com', '123')"
		if _, err = pool.Exec(ctx, stmt, userID); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		stmt = "INSERT INTO user_roles (role_name, user_id) VALUES ('Admin', $1)"
		if _, err = pool.Exec(ctx, stmt, userID); err != nil {
			t.Fatalf("failed to insert user roles: %v", err)
		}

		sessionID := uuid.New()
		expires := time.Now().Add(10 * time.Minute)
		stmt = "INSERT INTO sessions (id, user_id, expires) VALUES ($1, $2, $3)"
		if _, err = pool.Exec(ctx, stmt, sessionID, userID, expires); err != nil {
			t.Fatalf("failed to insert user roles: %v", err)
		}

		responseWriter := httptest.NewRecorder()
		authnHandler := authn.NewMockHandler(t)
		authnHandler.EXPECT().
			SetSessionCookie(responseWriter, mock.MatchedBy(func(extendedSession *session.Session) bool {
				return extendedSession.ID == sessionID && extendedSession.Expires.After(expires)
			})).Once()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			if actor == nil {
				t.Fatalf("expected actor, got nil")
			}

			if actor.User.GetID() != userID {
				t.Errorf("expected user ID %q, got %q", userID.String(), actor.User.GetID().String())
			}

			if len(actor.Roles) != 1 || actor.Roles[0].Name != "Admin" {
				t.Errorf("expected Admin role, got %#v", actor.Roles[0])
			}
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})

		oauth2Auth := middleware.Oauth2Authentication(authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie with matching expired session", func(t *testing.T) {
		ctx, pool := setup(t)

		userID := uuid.New()
		stmt := "INSERT INTO users (id, name, email, external_id) VALUES ($1, 'User 1', 'user1@example.com', '123')"
		if _, err = pool.Exec(ctx, stmt, userID); err != nil {
			t.Fatalf("failed to insert user: %v", err)
		}

		sessionID := uuid.New()
		expires := time.Now().Add(-10 * time.Second)
		stmt = "INSERT INTO sessions (id, user_id, expires) VALUES ($1, $2, $3)"
		if _, err = pool.Exec(ctx, stmt, sessionID, userID, expires); err != nil {
			t.Fatalf("failed to insert user roles: %v", err)
		}

		responseWriter := httptest.NewRecorder()
		authnHandler := authn.NewMockHandler(t)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := authz.ActorFromContext(r.Context()); actor != nil {
				t.Errorf("unexpected actor: %v", actor)
			}
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})

		oauth2Auth := middleware.Oauth2Authentication(authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)

		if _, err := session.Get(ctx, sessionID); err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
