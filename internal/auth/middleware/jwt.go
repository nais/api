package middleware

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jws"
	"github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
	"github.com/zitadel/oidc/v3/pkg/client"
)

const acceptableClockSkew = 3 * time.Second

type jwtAuth struct {
	issuer       string
	audience     string
	zitadelOrgID string
	jwksURL      string
	jwksCache    *jwk.Cache
	log          logrus.FieldLogger
	now          clock
}

type clock func() time.Time

func (c clock) Now() time.Time {
	return c()
}

func (j *jwtAuth) validate(ctx context.Context, token string) (jwt.Token, error) {
	jwks, err := j.jwksCache.Lookup(ctx, j.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("provider: fetching jwks: %w", err)
	}

	parseOpts := []jwt.ParseOption{
		jwt.WithKeySet(jwks),
		jwt.WithAcceptableSkew(acceptableClockSkew),
		jwt.WithClock(j.now),
		// No need to add issuer and audience during parsing, as they are
		// validated in the Validate method
	}

	t, err := jwt.ParseString(token, parseOpts...)
	if err != nil {
		if errors.Is(err, jws.VerifyError()) {
			_, err := j.jwksCache.Refresh(ctx, j.jwksURL)
			if err != nil {
				return nil, fmt.Errorf("provider: refreshing jwks during parsing: %w", err)
			}
		}
		return nil, fmt.Errorf("provider: parsing token: %w", err)
	}

	return t, jwt.Validate(t,
		jwt.WithAcceptableSkew(acceptableClockSkew),
		jwt.WithIssuer(j.issuer),
		jwt.WithAudience(j.audience),
		jwt.WithClaimValue("urn:zitadel:iam:user:resourceowner:id", j.zitadelOrgID),
		jwt.WithClock(j.now),
	)
}

func (j *jwtAuth) handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		token, ok := BearerAuth(r)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()
		tok, err := j.validate(ctx, token)
		if err != nil {
			j.log.WithError(err).Debug("failed to validate token")
			next.ServeHTTP(w, r)
			return
		}

		externalID, ok := tok.Subject()
		if !ok {
			j.log.Debug("failed to get subject from token")
			next.ServeHTTP(w, r)
			return
		}

		usr, err := user.GetByExternalID(ctx, externalID)
		if err != nil {
			j.log.WithError(err).Debug("failed to get user by external id")
			next.ServeHTTP(w, r)
			return
		}

		roles, err := authz.ForUser(ctx, usr.UUID)
		if err != nil {
			j.log.WithError(err).Debug("failed to get roles for user")
			next.ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r.WithContext(authz.ContextWithActor(ctx, usr, roles)))
	}
	return http.HandlerFunc(fn)
}

func JWTAuthentication(ctx context.Context, issuer, audience, zitadelOrgID string, log logrus.FieldLogger) (func(next http.Handler) http.Handler, error) {
	if issuer == "" {
		return nil, errors.New("issuer is required")
	}
	if audience == "" {
		return nil, errors.New("audience is required")
	}
	if zitadelOrgID == "" {
		return nil, errors.New("zitadelOrgID is required")
	}

	client, err := client.Discover(ctx, issuer, http.DefaultClient)
	if err != nil {
		return nil, fmt.Errorf("discovering oidc client: %w", err)
	}

	httpClient := httprc.NewClient()
	cache, err := jwk.NewCache(ctx, httpClient)
	if err != nil {
		return nil, fmt.Errorf("creating jwks cache: %w", err)
	}

	if err := cache.Register(ctx, client.JwksURI); err != nil {
		return nil, fmt.Errorf("registering jwks provider uri to cache: %w", err)
	}

	auth := jwtAuth{
		jwksURL:      client.JwksURI,
		jwksCache:    cache,
		issuer:       issuer,
		audience:     audience,
		zitadelOrgID: zitadelOrgID,
		log:          log,
		now:          time.Now,
	}

	return auth.handler, nil
}
