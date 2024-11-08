package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/user"
)

func TestRequireAuthenticatedUser(t *testing.T) {
	t.Run("no actor in context", func(t *testing.T) {
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Fatalf("unexpected call to next handler")
		})
		responseWriter := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/query", nil)
		middleware.RequireAuthenticatedUser()(next).ServeHTTP(responseWriter, req)

		if responseWriter.Code != http.StatusUnauthorized {
			t.Errorf("unexpected status code %d, got: %d", http.StatusUnauthorized, responseWriter.Code)
		}
	})

	t.Run("no actor in context", func(t *testing.T) {
		actor := &user.User{
			UUID:       uuid.New(),
			Email:      "mail@example.com",
			Name:       "Some Name",
			ExternalID: "123",
		}
		ctx := authz.ContextWithActor(context.Background(), actor, nil)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("OK"))
		})
		responseWriter := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(ctx, http.MethodPost, "/query", nil)
		middleware.RequireAuthenticatedUser()(next).ServeHTTP(responseWriter, req)

		if responseWriter.Code != http.StatusOK {
			t.Errorf("unexpected status code %d, got: %d", http.StatusOK, responseWriter.Code)
		} else if b := responseWriter.Body.String(); b != "OK" {
			t.Errorf("expected body %q, got: %q", "OK", b)
		}
	})
}
