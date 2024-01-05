package middleware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/nais/api/internal/authz"
	"github.com/nais/api/internal/db"
	"github.com/nais/api/internal/sqlc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/api/idtoken"
)

func TestIapAuthentication(t *testing.T) {
	getRequest := func(ctx context.Context) *http.Request {
		req, _ := http.NewRequest(http.MethodPost, "/query", nil)
		return req.WithContext(ctx)
	}

	t.Run("Valid headers", func(t *testing.T) {
		ctx := context.Background()
		userID := uuid.New()
		user := &db.User{
			User: &sqlc.User{
				ID:    userID,
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		roles := []*db.Role{
			{RoleName: sqlc.RoleNameAdmin},
		}

		responseWriter := httptest.NewRecorder()

		database := db.NewMockDatabase(t)
		database.
			On("GetUserByEmail", ctx, user.Email).
			Return(user, nil).
			Once()
		database.
			On("GetUserRoles", ctx, user.ID).
			Return(roles, nil).
			Once()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.NotNil(t, actor)
			assert.Equal(t, user, actor.User)
			assert.Equal(t, roles, actor.Roles)
		})
		req := getRequest(ctx)
		req.Header.Set(iapAudienceHeader, "valid")
		req.Header.Set(iapEmailHeader, "accounts.google.com:user@example.com")

		validator := func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			if idToken == "valid" && audience == "audience" {
				return &idtoken.Payload{
					Issuer:   iapIssuer,
					IssuedAt: time.Now().Unix(),
					Expires:  time.Now().Add(30 * time.Second).Unix(),
				}, nil
			}
			return nil, nil
		}
		middleware := iapAuthentication(database, "audience", validator)
		middleware(next).ServeHTTP(responseWriter, req)
	})

	t.Run("User not found", func(t *testing.T) {
		ctx := context.Background()
		responseWriter := httptest.NewRecorder()

		database := db.NewMockDatabase(t)
		database.
			On("GetUserByEmail", ctx, mock.Anything).
			Return(nil, pgx.ErrNoRows).
			Once()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(ctx)
		req.Header.Set(iapAudienceHeader, "valid")
		req.Header.Set(iapEmailHeader, "accounts.google.com:user@example.com")

		validator := func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			if idToken == "valid" && audience == "audience" {
				return &idtoken.Payload{
					Issuer:   iapIssuer,
					IssuedAt: time.Now().Unix(),
					Expires:  time.Now().Add(30 * time.Second).Unix(),
				}, nil
			}
			return nil, nil
		}
		middleware := iapAuthentication(database, "audience", validator)
		middleware(next).ServeHTTP(responseWriter, req)
	})

	t.Run("invalid token", func(t *testing.T) {
		ctx := context.Background()
		responseWriter := httptest.NewRecorder()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(ctx)

		validator := func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
			return nil, fmt.Errorf("invalid token")
		}
		middleware := iapAuthentication(nil, "audience", validator)
		middleware(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Token issues", func(t *testing.T) {
		tests := map[string]*idtoken.Payload{
			"expired": {
				Issuer:   iapIssuer,
				IssuedAt: time.Now().Add(-1 * time.Minute).Unix(),
				Expires:  time.Now().Add(-30 * time.Second).Unix(),
			},
			"not yet valid": {
				Issuer:   iapIssuer,
				IssuedAt: time.Now().Add(1 * time.Minute).Unix(),
				Expires:  time.Now().Add(30 * time.Second).Unix(),
			},
			"wrong issuer": {
				Issuer:   "https://example.com",
				IssuedAt: time.Now().Unix(),
				Expires:  time.Now().Add(30 * time.Second).Unix(),
			},
		}

		for name, payload := range tests {
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				responseWriter := httptest.NewRecorder()

				next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					actor := authz.ActorFromContext(r.Context())
					assert.Nil(t, actor)
				})
				req := getRequest(ctx)
				req.Header.Set(iapAudienceHeader, "valid")
				req.Header.Set(iapEmailHeader, "accounts.google.com:user@example.com")

				validator := func(ctx context.Context, idToken, audience string) (*idtoken.Payload, error) {
					if idToken == "valid" && audience == "audience" {
						return payload, nil
					}
					return nil, nil
				}
				middleware := iapAuthentication(nil, "audience", validator)
				middleware(next).ServeHTTP(responseWriter, req)
			})
		}
	})
}
