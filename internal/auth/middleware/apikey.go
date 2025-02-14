package middleware

import (
	"net/http"
	"strings"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/serviceaccount"
)

// ApiKeyAuthentication will authenticate a service account from a token found in the authorization header
func ApiKeyAuthentication() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") || len(authHeader) < 8 {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			secret := authHeader[7:]
			sa, err := serviceaccount.GetByToken(ctx, secret)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			roles, err := authz.ForServiceAccount(ctx, sa.UUID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if err := serviceaccount.UpdateSecretLastUsedAt(ctx, secret); err != nil {
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, sa, roles)))
		}
		return http.HandlerFunc(fn)
	}
}
