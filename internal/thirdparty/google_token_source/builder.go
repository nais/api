package google_token_source

import (
	"context"
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"
	admin_directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
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

	return impersonate.CredentialsTokenSource(ctx, impersonateConfig, option.WithHTTPClient(
		&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)},
	))
}

func (g Builder) Admin(ctx context.Context) (oauth2.TokenSource, error) {
	return g.impersonateTokenSource(ctx, true, []string{
		admin_directory.AdminDirectoryUserReadonlyScope,
		admin_directory.AdminDirectoryGroupScope,
	})
}

func (g Builder) GCP(ctx context.Context) (oauth2.TokenSource, error) {
	return g.impersonateTokenSource(ctx, false, []string{
		cloudresourcemanager.CloudPlatformScope,
	})
}
