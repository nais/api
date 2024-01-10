package middleware

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/nais/api/internal/auth/authz"
	db "github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
	"google.golang.org/api/idtoken"
)

type (
	valfunc func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error)
)

const (
	iapAudienceHeader = "X-Goog-IAP-JWT-Assertion"
	iapEmailHeader    = "X-Goog-Authenticated-User-Email"
	iapIssuer         = "https://cloud.google.com/iap"
)

func IAPInsecureAuthentication(database db.Database) func(next http.Handler) http.Handler {
	return iapAuthentication(database, "", func(ctx context.Context, idToken string, audience string) (*idtoken.Payload, error) {
		return &idtoken.Payload{
			Issuer:   iapIssuer,
			Audience: audience,
			IssuedAt: time.Now().Unix(),
			Expires:  time.Now().Add(30 * time.Second).Unix(),
		}, nil
	})
}

// IAPAuthentication authenticates a request using IAP headers.
func IAPAuthentication(database db.Database, aud string) func(next http.Handler) http.Handler {
	return iapAuthentication(database, aud, idtoken.Validate)
}

func iapAuthentication(database db.Database, aud string, validator valfunc) func(h http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			iapJWT := r.Header.Get(iapAudienceHeader)

			payload, err := validator(r.Context(), iapJWT, aud)
			if err != nil {
				h.ServeHTTP(w, r)
				return
			}

			if time.Unix(payload.IssuedAt, 0).After(time.Now().Add(30 * time.Second)) {
				h.ServeHTTP(w, r)
				return
			}

			if time.Unix(payload.Expires, 0).Before(time.Now()) {
				h.ServeHTTP(w, r)
				return
			}

			if payload.Issuer != iapIssuer {
				h.ServeHTTP(w, r)
				return
			}

			email := r.Header.Get(iapEmailHeader)
			_, email, _ = strings.Cut(email, ":")
			ctx := r.Context()

			user, err := database.GetUserByEmail(ctx, email)
			if err != nil {
				h.ServeHTTP(w, r)
				return
			}

			roles, err := database.GetUserRoles(ctx, user.ID)
			if err != nil {
				h.ServeHTTP(w, r)
				return
			}
			isAdmin := slices.ContainsFunc(roles, func(role *authz.Role) bool {
				return role.RoleName == sqlc.RoleNameAdmin
			})
			user.IsAdmin = &isAdmin
			ctx = authz.ContextWithActor(r.Context(), user, roles)
			h.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
