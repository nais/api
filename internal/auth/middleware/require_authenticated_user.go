package middleware

import (
	"fmt"
	"net/http"

	"github.com/nais/api/internal/auth/authz"
)

func RequireAuthenticatedUser() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			if authz.ActorFromContext(r.Context()) == nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = fmt.Fprintln(w, `{"errors": [{"message": "Unauthorized"}]}`)
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
