package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	db "github.com/nais/api/internal/database"
)

// Oauth2Authentication If the request has a session cookie, look up the session from the store, and if it exists, try
// to load the user with the email address stored in the session.
func Oauth2Authentication(database db.Database, authHandler authn.Handler) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(authn.SessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			sessionID, err := uuid.Parse(cookie.Value)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			session, err := database.GetSessionByID(ctx, sessionID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if !session.Expires.Valid || session.Expires.Time.Before(time.Now()) {
				_ = database.DeleteSession(ctx, sessionID)
				next.ServeHTTP(w, r)
				return
			}

			user, err := database.GetUserByID(ctx, session.UserID)
			if err != nil {
				_ = database.DeleteSession(ctx, sessionID)
				next.ServeHTTP(w, r)
				return
			}

			roles, err := database.GetUserRoles(ctx, user.ID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// extend the session every time the user does something
			session, err = database.ExtendSession(ctx, sessionID)
			if err != nil {
				_ = database.DeleteSession(ctx, sessionID)
				next.ServeHTTP(w, r)
				return
			}

			authHandler.SetSessionCookie(w, session)
			ctx = authz.ContextWithActor(r.Context(), user, roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}
