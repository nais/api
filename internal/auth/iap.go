package auth

import (
	"fmt"
	"net/http"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
)

// InsecureUserHeader returns a middleware that sets the email address of the authenticated user to the given value
func InsecureUserHeader(db database.Database) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			email := r.Header.Get("X-User-Email")
			if email == "" {
				next.ServeHTTP(w, r)
				return
			}

			usr, err := db.GetUserByEmail(ctx, email)
			if err != nil {
				http.Error(w, jsonError(fmt.Sprintf("User with email %q not found", email)), http.StatusUnauthorized)
				return
			}

			roles, err := db.GetUserRoles(ctx, usr.ID)
			if err != nil {
				http.Error(w, jsonError(fmt.Sprintf("Unable to get user roles for user with email %q", email)), http.StatusUnauthorized)
				return
			}

			r = r.WithContext(authz.ContextWithActor(ctx, usr, roles))
			next.ServeHTTP(w, r)
		})
	}
}

// jsonError returns a JSON error message
func jsonError(msg string) string {
	return fmt.Sprintf(`{"error": %q}`, msg)
}
