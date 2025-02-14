package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/serviceaccount"
)

type StaticServiceAccount struct {
	Name   string                     `json:"name"`
	Roles  []StaticServiceAccountRole `json:"roles"`
	APIKey string                     `json:"apiKey"`
}

type StaticServiceAccountRole struct {
	Name string `json:"name"`
}

var deprecatedRoles = []string{"Admin", "Team viewer", "User viewer"}

const naisServiceAccountPrefix = "nais-"

type StaticServiceAccounts []StaticServiceAccount

var _ json.Unmarshaler = (*StaticServiceAccounts)(nil)

func (s *StaticServiceAccounts) UnmarshalJSON(value []byte) error {
	if len(value) == 0 {
		return nil
	}

	serviceAccounts := make([]StaticServiceAccount, 0)
	err := json.NewDecoder(bytes.NewReader(value)).Decode(&serviceAccounts)
	if err != nil {
		return err
	}

	for _, serviceAccount := range serviceAccounts {
		if !strings.HasPrefix(serviceAccount.Name, naisServiceAccountPrefix) {
			return fmt.Errorf("service account is missing required %q prefix: %q", naisServiceAccountPrefix, serviceAccount.Name)
		}

		if serviceAccount.APIKey == "" {
			return fmt.Errorf("service account is missing an API key: %q", serviceAccount.Name)
		}
	}

	*s = serviceAccounts
	return nil
}

// setupStaticServiceAccounts will create a set of service accounts with roles and API keys.
func setupStaticServiceAccounts(ctx context.Context, pool *pgxpool.Pool, serviceAccounts StaticServiceAccounts) error {
	ctx = database.NewLoaderContext(ctx, pool)
	ctx = serviceaccount.NewLoaderContext(ctx, pool)
	ctx = authz.NewLoaderContext(ctx, pool)

	return database.Transaction(ctx, func(ctx context.Context) error {
		if err := serviceaccount.DeleteStaticServiceAccounts(ctx); err != nil {
			return err
		}

		for _, serviceAccountFromInput := range serviceAccounts {
			roles := make([]string, 0)
			for _, r := range serviceAccountFromInput.Roles {
				if slices.Contains(deprecatedRoles, r.Name) {
					continue
				}

				roles = append(roles, r.Name)
			}
			err := serviceaccount.CreateStaticServiceAccount(ctx, serviceAccountFromInput.Name, roles, serviceAccountFromInput.APIKey)
			if err != nil {
				return err
			}
		}

		return nil
	})
}
