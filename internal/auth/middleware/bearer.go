// Borrowed and adapted from https://github.com/golang/go/issues/70522

package middleware

import (
	"net/http"
	"strings"
)

// BearerAuth returns the token provided in the request's
// Authorization header, if the request uses the Bearer authentication scheme.
func BearerAuth(r *http.Request) (token string, ok bool) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", false
	}

	const prefix = "Bearer "
	if len(auth) < len(prefix) || !strings.EqualFold(auth[:len(prefix)], prefix) {
		return "", false
	}

	token = auth[len(prefix):]
	if len(token) == 0 {
		return "", false
	}
	return token, true
}
