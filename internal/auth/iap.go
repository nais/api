package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

const (
	contextEmail        contextKey = 1
	contextIAPAssertion contextKey = 2
	contextIAPEmail     contextKey = 3
)

type (
	contextKey int
	Middleware func(next http.Handler) http.Handler
)

// StaticUser returns a middleware that sets the email address of the authenticated user to the given value
func StaticUser(email string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := SetIAP(r.Context(), "dummy", ":"+email)
			ctx = context.WithValue(ctx, contextEmail, email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ValidateIAPJWT returns a middleware that validates the X-Goog-IAP-JWT-Assertion header and sets the email address of
// the authenticated user to the value of the X-Goog-Authenticated-User-Email header
func ValidateIAPJWT(aud string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			iapJWT := r.Header.Get("X-Goog-IAP-JWT-Assertion")

			payload, err := idtoken.Validate(r.Context(), iapJWT, aud)
			if err != nil {
				http.Error(w, jsonError("Invalid JWT token"), http.StatusUnauthorized)
				return
			}

			if time.Unix(payload.IssuedAt, 0).After(time.Now().Add(30 * time.Second)) {
				http.Error(w, jsonError("JWT token is in the future"), http.StatusUnauthorized)
				return
			}

			if payload.Issuer != "https://cloud.google.com/iap" {
				http.Error(w, jsonError("Invalid JWT token issuer"), http.StatusUnauthorized)
				return
			}

			emailh := r.Header.Get("X-Goog-Authenticated-User-Email")
			_, email, _ := strings.Cut(emailh, ":")
			ctx := context.WithValue(r.Context(), contextEmail, email)
			ctx = SetIAP(ctx, iapJWT, emailh)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetEmail returns the email address of the authenticated user that is stored in the context
func GetEmail(ctx context.Context) (string, error) {
	email, ok := ctx.Value(contextEmail).(string)
	if !ok || email == "" {
		return "", fmt.Errorf("no email in context")
	}
	return email, nil
}

// GetIAPAssertion returns the IAP JWT assertion that is stored in the context
func GetIAPAssertion(ctx context.Context) (string, error) {
	iapAssertion, ok := ctx.Value(contextIAPAssertion).(string)
	if !ok || iapAssertion == "" {
		return "", fmt.Errorf("no IAP assertion in context")
	}
	return iapAssertion, nil
}

// GetIAPEmail returns the IAP email that is stored in the context
func GetIAPEmail(ctx context.Context) (string, error) {
	email, ok := ctx.Value(contextIAPEmail).(string)
	if !ok || email == "" {
		return "", fmt.Errorf("no IAP email in context")
	}
	return email, nil
}

// setIAP sets the IAP headers
func SetIAP(ctx context.Context, assertion, email string) context.Context {
	ctx = context.WithValue(ctx, contextIAPAssertion, assertion)
	ctx = context.WithValue(ctx, contextIAPEmail, email)
	return ctx
}

// jsonError returns a JSON error message
func jsonError(msg string) string {
	return fmt.Sprintf(`{"error": "%s"}`, msg)
}
