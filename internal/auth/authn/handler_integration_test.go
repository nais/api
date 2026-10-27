//go:build integration_test

package authn

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/session"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/usersync/usersyncsql"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/oauth2"
)

// callbackOAuth verifies the claims using the normal OIDC verifier with a test
// key set. Only signature verification is replaced; claims are still decoded by
// go-oidc, including its otherwise private IDToken claims field.
type callbackOAuth struct {
	zitadel bool
	subject string
	email   string
}

func (c callbackOAuth) IsZitadel() bool { return c.zitadel }

func (c callbackOAuth) AuthCodeURL(string, ...oauth2.AuthCodeOption) string { return "/login" }

func (c callbackOAuth) Exchange(context.Context, string, ...oauth2.AuthCodeOption) (*oauth2.Token, error) {
	return (&oauth2.Token{}).WithExtra(map[string]any{IDTokenKey: "test-token"}), nil
}

func (c callbackOAuth) Verify(ctx context.Context, _ string) (*oidc.IDToken, error) {
	payload := fmt.Sprintf(`{"iss":"https://auth.nais.io","aud":"console","sub":%q,"email":%q,"exp":%d}`, c.subject, c.email, time.Now().Add(time.Hour).Unix())
	raw := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + ".c2ln"
	return oidc.NewVerifier("https://auth.nais.io", callbackKeySet{payload: []byte(payload)}, &oidc.Config{ClientID: "console"}).Verify(ctx, raw)
}

type callbackKeySet struct{ payload []byte }

func (k callbackKeySet) VerifySignature(context.Context, string) ([]byte, error) {
	return k.payload, nil
}

func TestCallbackUsesEmail(t *testing.T) {
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
	u, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Nais viewer", Email: "viewer@nais.io", ExternalID: "nais-subject"})
	if err != nil {
		t.Fatal(err)
	}
	tenant, err := q.Create(ctx, usersyncsql.CreateParams{Name: "Tenant viewer", Email: "viewer@nav.no", ExternalID: "tenant-subject"})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		config     callbackOAuth
		wantUserID uuid.UUID
	}{
		{name: "nais.io user", config: callbackOAuth{zitadel: true, subject: u.ExternalID, email: u.Email}, wantUserID: u.ID},
		{name: "tenant user", config: callbackOAuth{zitadel: true, subject: tenant.ExternalID, email: tenant.Email}, wantUserID: tenant.ID},
		{name: "nais.io user matched by email regardless of subject", config: callbackOAuth{zitadel: true, subject: "new-nais-subject", email: u.Email}, wantUserID: u.ID},
		{name: "tenant user matched by email regardless of subject", config: callbackOAuth{zitadel: true, subject: "new-tenant-subject", email: tenant.Email}, wantUserID: tenant.ID},
		{name: "known nais.io subject with unknown email", config: callbackOAuth{zitadel: true, subject: u.ExternalID, email: "unknown@nais.io"}},
		{name: "known tenant subject with unknown email", config: callbackOAuth{zitadel: true, subject: tenant.ExternalID, email: "unknown@nav.no"}},
		{name: "unsynced user", config: callbackOAuth{zitadel: true, subject: "unsynced", email: "unsynced@nais.io"}},
		{name: "non-Zitadel login still uses email", config: callbackOAuth{subject: "google-subject", email: tenant.Email}, wantUserID: tenant.ID},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := database.NewLoaderContext(t.Context(), pool)
			ctx = user.NewLoaderContext(ctx, pool)
			ctx = session.NewLoaderContext(ctx, pool)
			req := httptest.NewRequest(http.MethodGet, "/oauth2/callback?code=code&state=state", nil).WithContext(ctx)
			req.AddCookie(&http.Cookie{Name: OAuthStateCookie, Value: "state"})
			rec := httptest.NewRecorder()
			New(tc.config, log).Callback(rec, req)
			if rec.Code != http.StatusFound {
				t.Fatalf("expected redirect, got %d: %s", rec.Code, rec.Body.String())
			}
			if tc.wantUserID == uuid.Nil {
				if got := rec.Header().Get("Location"); got != "/?error=unknown-user" {
					t.Fatalf("expected unknown user, got %q", got)
				}
				for _, cookie := range rec.Result().Cookies() {
					if cookie.Name == SessionCookieName {
						t.Fatal("unexpected session for an unknown identity")
					}
				}
				return
			}
			if got := rec.Header().Get("Location"); got != "/" {
				t.Fatalf("expected successful login, got %q", got)
			}
			for _, cookie := range rec.Result().Cookies() {
				if cookie.Name != SessionCookieName {
					continue
				}
				sess, err := session.Get(ctx, uuid.MustParse(cookie.Value))
				if err != nil || sess.UserID != tc.wantUserID {
					t.Fatalf("session belongs to wrong identity: %v, %v", sess, err)
				}
				return
			}
			t.Fatal("missing session cookie")
		})
	}
}
