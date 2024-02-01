package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/auth/middleware"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/stretchr/testify/assert"
)

func TestOauth2Authentication(t *testing.T) {
	t.Run("No cookie in request", func(t *testing.T) {
		db := database.NewMockDatabase(t)
		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(context.Background())
		oauth2Auth := middleware.Oauth2Authentication(db, authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Invalid cookie value", func(t *testing.T) {
		ctx := context.Background()
		db := database.NewMockDatabase(t)
		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: "unknown-session-key",
		})
		oauth2Auth := middleware.Oauth2Authentication(db, authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie, no session in store", func(t *testing.T) {
		ctx := context.Background()
		db := database.NewMockDatabase(t)
		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(ctx)
		sessionID := uuid.New()
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})
		notFoundErr := errors.New("not found")
		db.EXPECT().
			GetSessionByID(ctx, sessionID).
			Return(nil, notFoundErr).
			Once()
		oauth2Auth := middleware.Oauth2Authentication(db, authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie with matching session", func(t *testing.T) {
		ctx := context.Background()
		sessionID := uuid.New()
		userID := uuid.New()
		user := &database.User{
			User: &gensql.User{
				ID:    userID,
				Email: "user@example.com",
				Name:  "User Name",
			},
		}
		roles := []*authz.Role{
			{RoleName: gensql.RoleNameAdmin},
		}
		session := &database.Session{Session: &gensql.Session{
			ID:     sessionID,
			UserID: userID,
			Expires: pgtype.Timestamptz{
				Time:  time.Now().Add(10 * time.Second),
				Valid: true,
			},
		}}
		extendedSession := &database.Session{Session: &gensql.Session{
			ID:     sessionID,
			UserID: userID,
			Expires: pgtype.Timestamptz{
				Time:  time.Now().Add(30 * time.Minute),
				Valid: true,
			},
		}}

		responseWriter := httptest.NewRecorder()

		authnHandler := authn.NewMockHandler(t)
		authnHandler.EXPECT().
			SetSessionCookie(responseWriter, extendedSession)

		db := database.NewMockDatabase(t)
		db.EXPECT().
			GetSessionByID(ctx, sessionID).
			Return(session, nil).
			Once()
		db.EXPECT().
			GetUserByID(ctx, userID).
			Return(user, nil).
			Once()
		db.EXPECT().
			GetUserRoles(ctx, user.ID).
			Return(roles, nil).
			Once()
		db.EXPECT().
			ExtendSession(ctx, sessionID).
			Return(extendedSession, nil).
			Once()

		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.NotNil(t, actor)
			assert.Equal(t, user, actor.User)
			assert.Equal(t, roles, actor.Roles)
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})

		oauth2Auth := middleware.Oauth2Authentication(db, authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie with matching expired session", func(t *testing.T) {
		ctx := context.Background()
		sessionID := uuid.New()
		userID := uuid.New()
		db := database.NewMockDatabase(t)
		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})
		session := &database.Session{Session: &gensql.Session{
			ID:     sessionID,
			UserID: userID,
			Expires: pgtype.Timestamptz{
				Time:  time.Now().Add(-10 * time.Second),
				Valid: true,
			},
		}}
		db.EXPECT().
			GetSessionByID(ctx, sessionID).
			Return(session, nil).
			Once()
		db.EXPECT().
			DeleteSession(ctx, sessionID).
			Return(nil).
			Once()

		oauth2Auth := middleware.Oauth2Authentication(db, authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})

	t.Run("Valid cookie with matching session with invalid userID in session", func(t *testing.T) {
		ctx := context.Background()
		sessionID := uuid.New()
		userID := uuid.New()
		db := database.NewMockDatabase(t)
		authnHandler := authn.NewMockHandler(t)
		responseWriter := httptest.NewRecorder()
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			actor := authz.ActorFromContext(r.Context())
			assert.Nil(t, actor)
		})
		req := getRequest(ctx)
		req.AddCookie(&http.Cookie{
			Name:  authn.SessionCookieName,
			Value: sessionID.String(),
		})
		session := &database.Session{Session: &gensql.Session{
			ID:     sessionID,
			UserID: userID,
			Expires: pgtype.Timestamptz{
				Time:  time.Now().Add(10 * time.Second),
				Valid: true,
			},
		}}
		db.EXPECT().
			GetSessionByID(ctx, sessionID).
			Return(session, nil).
			Once()
		db.EXPECT().
			GetUserByID(ctx, userID).
			Return(nil, errors.New("not found")).
			Once()
		db.EXPECT().
			DeleteSession(ctx, sessionID).
			Return(nil).
			Once()

		oauth2Auth := middleware.Oauth2Authentication(db, authnHandler)
		oauth2Auth(next).ServeHTTP(responseWriter, req)
	})
}
