package usersync

import (
	"context"

	"golang.org/x/oauth2"
	admin_directory "google.golang.org/api/admin/directory/v1"
	"google.golang.org/api/impersonate"
)

type tokenSource struct {
	serviceAccountEmail string
	subjectEmail        string
}

func newTokenSource(serviceAccount, subjectEmail string) (*tokenSource, error) {
	return &tokenSource{
		serviceAccountEmail: serviceAccount,
		subjectEmail:        subjectEmail,
	}, nil
}

func (g tokenSource) impersonateTokenSource(ctx context.Context, delegate bool, scopes []string) (oauth2.TokenSource, error) {
	impersonateConfig := impersonate.CredentialsConfig{
		TargetPrincipal: g.serviceAccountEmail,
		Scopes:          scopes,
	}
	if delegate {
		impersonateConfig.Subject = g.subjectEmail
	}

	// Otel transport is added by the library
	return impersonate.CredentialsTokenSource(ctx, impersonateConfig)
}

func (g tokenSource) Admin(ctx context.Context) (oauth2.TokenSource, error) {
	return g.impersonateTokenSource(ctx, true, []string{
		admin_directory.AdminDirectoryUserReadonlyScope,
		admin_directory.AdminDirectoryGroupScope,
	})
}
