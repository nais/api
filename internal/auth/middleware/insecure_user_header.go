package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/user"
)

// InsecureUserHeader returns a middleware that sets the email address of the authenticated user from the x-user-email
// header. This middleware is intended for local development and testing purposes only.
func InsecureUserHeader() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			email := r.Header.Get("X-User-Email")
			if email == "" {
				// Hack to allow introspection locally without a user
				body, err := io.ReadAll(r.Body)
				if err != nil {
					http.Error(w, jsonError("Unable to read request body"), http.StatusBadRequest)
					return
				}

				// Recreate request with body
				r.Body = io.NopCloser(bytes.NewReader(body))

				if bytes.Contains(body, []byte("query IntrospectionQuery {")) {
					email = "dev.usersen@example.com"
				} else {
					next.ServeHTTP(w, r)
					return
				}
			}

			u, err := user.GetByEmail(ctx, email)
			if err != nil {
				http.Error(w, jsonError(fmt.Sprintf("User with email %q not found", email)), http.StatusUnauthorized)
				return
			}

			roles, err := role.ForUser(ctx, u.UUID)
			if err != nil {
				http.Error(w, jsonError(fmt.Sprintf("Unable to get user roles for user with email %q", email)), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, u, roles)))
		})
	}
}

// jsonError returns a JSON error message
func jsonError(msg string) string {
	return fmt.Sprintf(`{"error": %q}`, msg)
}
