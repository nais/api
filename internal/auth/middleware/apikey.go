package middleware

import (
	"net/http"
	"strings"

	db "github.com/nais/api/internal/database"

	"github.com/nais/api/internal/auth/authz"
)

// ApiKeyAuthentication If the request has an authorization header, we will try to pull the service account who owns it
// from the database and put the account into the context.
func ApiKeyAuthentication(database db.Database) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || len(authHeader) < 8 {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			serviceAccount, err := database.GetServiceAccountByApiKey(ctx, authHeader[7:])
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			roles, err := database.GetServiceAccountRoles(ctx, serviceAccount.ID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx = authz.ContextWithActor(ctx, serviceAccount, roles)
			next.ServeHTTP(w, r.WithContext(ctx))
		}
		return http.HandlerFunc(fn)
	}
}
