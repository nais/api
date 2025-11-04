package middleware

import (
	"fmt"
	"net/http"
)

// PreSharedKeyAuthentication will authenticate a request against a pre shared key
func PreSharedKeyAuthentication(preSharedKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			// If no pre shared key is set, don't allow access
			if preSharedKey == "" {
				handleError(w)
				return
			}

			secret, ok := BearerAuth(r)
			if !ok {
				handleError(w)
				return
			}

			if secret != preSharedKey {
				handleError(w)
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func handleError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = fmt.Fprintln(w, `{"errors": [{"message": "Unauthorized"}]}`)
}
