package google_token_source

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/config"
	"golang.org/x/oauth2"
	admin_directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/cloudresourcemanager/v3"
	"google.golang.org/api/impersonate"
)

type Builder struct {
	serviceAccountEmail string
	subjectEmail        string
}

func NewFromConfig(cfg *config.TeamsConfig) (*Builder, error) {
	if cfg.GoogleManagementProjectID == "" {
		return nil, fmt.Errorf("missing required configuration: API_BACKEND_GOOGLE_MANAGEMENT_PROJECT_ID")
	}

	if cfg.TenantDomain == "" {
		return nil, fmt.Errorf("missing required configuration: API_BACKEND_TENANT_DOMAIN")
	}

	return &Builder{
		serviceAccountEmail: fmt.Sprintf("console@%s.iam.gserviceaccount.com", cfg.GoogleManagementProjectID),
		subjectEmail:        "nais-console@" + cfg.TenantDomain,
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

	return impersonate.CredentialsTokenSource(ctx, impersonateConfig)
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
