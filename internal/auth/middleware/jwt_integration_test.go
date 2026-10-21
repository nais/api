//go:build integration_test

package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/usersync/usersyncsql"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func TestJWTAuthenticationUsers(t *testing.T) {
	ctx := t.Context()
	log, _ := logrustest.NewNullLogger()
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"), postgres.WithUsername("test"), postgres.WithPassword("test"),
		postgres.BasicWaitStrategies())
	if err != nil {
		t.Fatal(err)
	}
	testcontainers.CleanupContainer(t, container)
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := database.NewPool(ctx, dsn, log, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	q := usersyncsql.New(pool)
	tenant, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Tenant user", Email: "user@nav.no", ExternalID: "tenant-subject"})
	if err != nil {
		t.Fatal(err)
	}
	if err := q.AssignGlobalRole(ctx, usersyncsql.AssignGlobalRoleParams{UserID: tenant.ID, RoleName: "Team creator"}); err != nil {
		t.Fatal(err)
	}
	nais, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Nais viewer", Email: "user@nais.io", ExternalID: "nais-subject"})
	if err != nil {
		t.Fatal(err)
	}

	const issuer = "https://auth.nais.io"
	keys, err := newJwkSet("key")
	if err != nil {
		t.Fatal(err)
	}
	cache, err := jwk.NewCache(ctx, httprc.NewClient(httprc.WithHTTPClient(mockJWKSetClient(keys))))
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.Register(ctx, issuer); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	for _, tc := range []struct {
		name       string
		orgID      string
		subject    string
		tenantOnly bool
		wantUserID uuid.UUID
		wantRoles  int
	}{
		{name: "tenant user", orgID: "tenant-org", subject: tenant.ExternalID, wantUserID: tenant.ID, wantRoles: 1},
		{name: "nais.io viewer", orgID: "nais-org", subject: nais.ExternalID, wantUserID: nais.ID},
		{name: "nais.io disabled", orgID: "nais-org", subject: nais.ExternalID, tenantOnly: true},
		{name: "unsynced nais.io user", orgID: "nais-org", subject: "unknown-subject"},
		{name: "unrelated organization", orgID: "other-org", subject: nais.ExternalID},
		{name: "missing subject", orgID: "nais-org"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := database.NewLoaderContext(t.Context(), pool)
			ctx = user.NewLoaderContext(ctx, pool)
			ctx = authz.NewLoaderContext(ctx, pool)
			orgIDs := []string{"tenant-org", "nais-org"}
			if tc.tenantOnly {
				orgIDs = orgIDs[:1]
			}
			mw := jwtAuth{
				issuer: issuer, audience: "cli", zitadelOrgIDs: orgIDs,
				jwksURL: issuer, jwksCache: cache, log: log,
				now: func() time.Time { return now },
			}
			// Like the current CLI access token, this token has no email claim.
			tok := token(now, time.Minute)
			tok.Set("iss", issuer)
			tok.Set("aud", "cli")
			tok.Set("urn:zitadel:iam:user:resourceowner:id", tc.orgID)
			if tc.subject != "" {
				tok.Set("sub", tc.subject)
			}
			signed, err := tok.sign(keys)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/graphql", nil).WithContext(ctx)
			req.Header.Set("Authorization", "Bearer "+signed)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actor := authz.ActorFromContext(r.Context())
				if tc.wantUserID == uuid.Nil {
					if actor != nil {
						t.Fatalf("unexpected authenticated actor: %v", actor)
					}
					return
				}
				if actor == nil || actor.User.GetID() != tc.wantUserID {
					t.Fatalf("expected user %s, got %v", tc.wantUserID, actor)
				}
				if actor.User.IsAdmin() || len(actor.Roles) != tc.wantRoles {
					t.Fatalf("unexpected privileges: admin=%v roles=%v", actor.User.IsAdmin(), actor.Roles)
				}
			})
			mw.handler(next).ServeHTTP(httptest.NewRecorder(), req)
		})
	}
}
