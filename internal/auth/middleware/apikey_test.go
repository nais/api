package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/authz"
	"github.com/nais/api/internal/db"
	"github.com/nais/api/internal/middleware"
	"github.com/nais/api/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestApiKeyAuthentication(t *testing.T) {
	t.Run("No authorization header", func(t *testing.T) {
		database := db.NewMockDatabase(t)
		responseWriter := httptest.NewRecorder()
		middleware := middleware.ApiKeyAuthentication(database)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(context.Background())
		middleware(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Unknown API key in header", func(t *testing.T) {
		database := db.NewMockDatabase(t)
		database.
			On("GetServiceAccountByApiKey", mock.Anything, "unknown").
			Return(nil, errors.New("user not found")).
			Once()
		responseWriter := httptest.NewRecorder()
		middleware := middleware.ApiKeyAuthentication(database)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(context.Background())
		req.Header.Set("Authorization", "Bearer unknown")
		middleware(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid API key", func(t *testing.T) {
		serviceAccount := &db.ServiceAccount{
			ServiceAccount: &sqlc.ServiceAccount{
				ID:   uuid.New(),
				Name: "service-account",
			},
		}
		roles := []*db.Role{
			{RoleName: sqlc.RoleNameAdmin},
		}

		database := db.NewMockDatabase(t)
		database.
			On("GetServiceAccountByApiKey", mock.Anything, "user1-key").
			Return(serviceAccount, nil).
			Once()
		database.
			On("GetServiceAccountRoles", mock.Anything, serviceAccount.ID).
			Return(roles, nil).
			Once()

		responseWriter := httptest.NewRecorder()
		middleware := middleware.ApiKeyAuthentication(database)
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.NotNil(t, actor)
			assert.Equal(t, serviceAccount, actor.User)
			assert.Equal(t, roles, actor.Roles)
		})
		req := getRequest(context.Background())
		req.Header.Set("Authorization", "Bearer user1-key")
		middleware(next).ServeHTTP(responseWriter, req)
	})
}

func getRequest(ctx context.Context) *http.Request {
	req, _ := http.NewRequest(http.MethodPost, "/query", nil)
	return req.WithContext(ctx)
}
