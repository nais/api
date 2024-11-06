package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/role/rolesql"
	"github.com/nais/api/internal/serviceaccount"
)

type StaticServiceAccount struct {
	Name   string                     `json:"name"`
	Roles  []StaticServiceAccountRole `json:"roles"`
	APIKey string                     `json:"apiKey"`
}

type StaticServiceAccountRole struct {
	Name rolesql.RoleName `json:"name"`
}

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

		if len(serviceAccount.Roles) == 0 {
			return fmt.Errorf("service account must have at least one role: %q", serviceAccount.Name)
		}

		if serviceAccount.APIKey == "" {
			return fmt.Errorf("service account is missing an API key: %q", serviceAccount.Name)
		}

		for _, r := range serviceAccount.Roles {
			if !r.Name.Valid() {
				return fmt.Errorf("invalid role name: %q for service account %q", r.Name, serviceAccount.Name)
			}
		}
	}

	*s = serviceAccounts
	return nil
}

// setupStaticServiceAccounts will create a set of service accounts with roles and API keys.
func setupStaticServiceAccounts(ctx context.Context, pool *pgxpool.Pool, serviceAccounts StaticServiceAccounts) error {
	ctx = database.NewLoaderContext(ctx, pool)
	ctx = serviceaccount.NewLoaderContext(ctx, pool)
	ctx = role.NewLoaderContext(ctx, pool)

	return database.Transaction(ctx, func(ctx context.Context) error {
		names := make(map[string]struct{})
		for _, serviceAccountFromInput := range serviceAccounts {
			names[serviceAccountFromInput.Name] = struct{}{}
			sa, err := serviceaccount.GetByName(ctx, serviceAccountFromInput.Name)
			if err != nil {
				sa, err = serviceaccount.Create(ctx, serviceAccountFromInput.Name)
				if err != nil {
					return err
				}
			}

			if err := serviceaccount.RemoveApiKeysFromServiceAccount(ctx, sa.UUID); err != nil {
				return err
			}

			existingRoles, err := role.ForServiceAccount(ctx, sa.UUID)
			if err != nil {
				return err
			}

			for _, r := range serviceAccountFromInput.Roles {
				if hasGlobalRoleRole(r.Name, existingRoles) {
					continue
				}

				if err := role.AssignGlobalRoleToServiceAccount(ctx, sa.UUID, r.Name); err != nil {
					return err
				}
			}

			if err := serviceaccount.CreateAPIKey(ctx, serviceAccountFromInput.APIKey, sa.UUID); err != nil {
				return err
			}
		}

		// remove all NAIS service accounts that is not present in the JSON input
		all, err := serviceaccount.List(ctx)
		if err != nil {
			return err
		}

		for _, sa := range all {
			if !strings.HasPrefix(sa.Name, naisServiceAccountPrefix) {
				continue
			}

			if _, shouldExist := names[sa.Name]; shouldExist {
				continue
			}

			if err := serviceaccount.Delete(ctx, sa.UUID); err != nil {
				return err
			}
		}

		return nil
	})
}

func hasGlobalRoleRole(roleName rolesql.RoleName, existingRoles []*role.Role) bool {
	for _, r := range existingRoles {
		if r.TargetTeamSlug != nil {
			continue
		}

		if roleName == r.Name {
			return true
		}
	}

	return false
}
