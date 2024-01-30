package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/stretchr/testify/mock"
)

func TestApiKeyAuthentication(t *testing.T) {
	t.Run("No authorization header", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		responseWriter := httptest.NewRecorder()
		apiKeyAuth := middleware.ApiKeyAuthentication(db)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			if actor != nil {
				t.Fatal("expected nil actor")
			}
		})
		req := getRequest(context.Background())
		apiKeyAuth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Unknown API key in header", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		db.
			On("GetServiceAccountByApiKey", mock.Anything, "unknown").
			Return(nil, errors.New("user not found")).
			Once()
		responseWriter := httptest.NewRecorder()
		apiKeyAuth := middleware.ApiKeyAuthentication(db)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			if actor != nil {
				t.Fatal("expected nil actor")
			}
		})
		req := getRequest(context.Background())
		req.Header.Set("Authorization", "Bearer unknown")
		apiKeyAuth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid API key", func(t *testing.T) {
		serviceAccount := &database.ServiceAccount{
			ServiceAccount: &gensql.ServiceAccount{
				ID:   uuid.New(),
				Name: "service-account",
			},
		}
		roles := []*authz.Role{
			{RoleName: gensql.RoleNameAdmin},
		}

		db := database.NewMockDatabase(t)
		db.
			On("GetServiceAccountByApiKey", mock.Anything, "user1-key").
			Return(serviceAccount, nil).
			Once()
		db.
			On("GetServiceAccountRoles", mock.Anything, serviceAccount.ID).
			Return(roles, nil).
			Once()

		responseWriter := httptest.NewRecorder()
		apiKeyAuth := middleware.ApiKeyAuthentication(db)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			if actor == nil {
				t.Fatal("expected actor")
			}
			want := &authz.Actor{
				User:  serviceAccount,
				Roles: roles,
			}

			if diff := cmp.Diff(want, actor); diff != "" {
				t.Errorf("diff: -want +got\n%s", diff)
			}
		})
		req := getRequest(context.Background())
		req.Header.Set("Authorization", "Bearer user1-key")
		apiKeyAuth(next).ServeHTTP(responseWriter, req)
	})
}

func getRequest(ctx context.Context) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/query", nil)
	return req.WithContext(ctx)
}
