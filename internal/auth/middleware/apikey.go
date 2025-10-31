package middleware

import (
	"net/http"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/serviceaccount"
)

// ApiKeyAuthentication will authenticate a service account from a token found in the authorization header
func ApiKeyAuthentication() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			secret, ok := BearerAuth(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			ctx := r.Context()
			sa, saToken, err := serviceaccount.GetBySecret(ctx, secret)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			roles, err := authz.ForServiceAccount(ctx, sa.UUID)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			if err := serviceaccount.UpdateTokenLastUsedAt(ctx, saToken.UUID); err != nil {
				next.ServeHTTP(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, sa, roles)))
		}
		return http.HandlerFunc(fn)
	}
}
