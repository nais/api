package middleware

import (
	"net/http"
	"strings"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/role"
	"github.com/nais/api/internal/v1/serviceaccount"
)

// ApiKeyAuthentication If the request has an authorization header, we will try to pull the service account who owns it
// from the database and put the account into the context.
func ApiKeyAuthentication() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || len(authHeader) < 8 {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			sa, err := serviceaccount.GetByApiKey(ctx, authHeader[7:])
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			roles, err := role.ForServiceAccount(ctx, sa.UUID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, sa, roles)))
		}
		return http.HandlerFunc(fn)
	}
}
