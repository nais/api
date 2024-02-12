package google_token_source

import (
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/oauth2"
	admin_directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/impersonate"
)

type Builder struct {
	serviceAccountEmail string
	subjectEmail        string
}

func New(googleManagementProjectID, tenantDomain string) (*Builder, error) {
	if googleManagementProjectID == "" {
		return nil, fmt.Errorf("empty googleManagementProjectID")
	}

	if tenantDomain == "" {
		return nil, fmt.Errorf("empty domain")
	}

	return &Builder{
		serviceAccountEmail: fmt.Sprintf("nais-api@%s.iam.gserviceaccount.com", googleManagementProjectID),
		subjectEmail:        "nais-admin@" + tenantDomain,
	}, nil
}

func (g Builder) impersonateTokenSource(ctx context.Context, delegate bool, scopes []string) (oauth2.TokenSource, error) {
	impersonateConfig := impersonate.CredentialsConfig{
		TargetPrincipal: g.serviceAccountEmail,
		Scopes:          scopes,
	}
	if delegate {
		impersonateConfig.Subject = g.subjectEmail
	}

	spew.Dump(impersonateConfig)

	// Otel transport is added by the library
	return impersonate.CredentialsTokenSource(ctx, impersonateConfig)
}

func (g Builder) Admin(ctx context.Context) (oauth2.TokenSource, error) {
	return g.impersonateTokenSource(ctx, true, []string{
		admin_directory.AdminDirectoryUserReadonlyScope,
		admin_directory.AdminDirectoryGroupScope,
	})
}
