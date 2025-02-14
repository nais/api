package middleware

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authn"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/session"
	"github.com/nais/api/internal/user"
)

// Oauth2Authentication If the request has a session cookie, look up the session from the store, and if it exists, try
// to load the user with the email address stored in the session.
func Oauth2Authentication(authHandler authn.Handler) func(next http.Handler) http.Handler {
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

			sess, err := session.Get(ctx, sessionID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if sess.HasExpired() {
				_ = session.Delete(ctx, sessionID)
				next.ServeHTTP(w, r)
				return
			}

			u, err := user.Get(ctx, sess.UserID)
			if err != nil {
				_ = session.Delete(ctx, sessionID)
				next.ServeHTTP(w, r)
				return
			}

			roles, err := authz.ForUser(ctx, u.UUID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// extend the session every time the user does something
			sess, err = session.Extend(ctx, sessionID)
			if err != nil {
				_ = session.Delete(ctx, sessionID)
				next.ServeHTTP(w, r)
				return
			}

			authHandler.SetSessionCookie(w, sess)
			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, u, roles)))
		}
		return http.HandlerFunc(fn)
	}
}
