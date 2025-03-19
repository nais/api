package authn

import (
	"context"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type OIDC struct {
	oauth2.Config
	provider *oidc.Provider
}

func NewOIDC(ctx context.Context, issuer, clientID, clientSecret, redirectURL string, additionalScopes []string) (*OIDC, error) {
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, err
	}

	return &OIDC{
		provider: provider,
		Config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  redirectURL,
			Scopes:       append([]string{oidc.ScopeOpenID, "profile", "email"}, additionalScopes...),
		},
	}, nil
}

func (o *OIDC) Verify(ctx context.Context, rawIDToken string) (*oidc.IDToken, error) {
	return o.provider.Verifier(&oidc.Config{ClientID: o.Config.ClientID}).Verify(ctx, rawIDToken)
}
