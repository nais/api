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
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/serviceaccount"
	"k8s.io/utils/ptr"
)

type StaticServiceAccount struct {
	Name   string                     `json:"name"`
	Roles  []StaticServiceAccountRole `json:"roles"`
	APIKey string                     `json:"apiKey"`
}

type StaticServiceAccountRole struct {
	Name string `json:"name"`
}

var deprecatedRoles = []string{"Team viewer", "User viewer"}

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
		names := make(map[string]struct{})
		for _, serviceAccountFromInput := range serviceAccounts {
			names[serviceAccountFromInput.Name] = struct{}{}
			sa, err := serviceaccount.GetByName(ctx, serviceAccountFromInput.Name)
			if err != nil {
				sa, err = serviceaccount.Create(ctx, serviceaccount.CreateServiceAccountInput{
					Name:        serviceAccountFromInput.Name,
					Description: "Static service account created by Nais",
				})
				if err != nil {
					return err
				}
			}

			if err := serviceaccount.RemoveApiKeysFromServiceAccount(ctx, sa.UUID); err != nil {
				return err
			}

			existingRoles, err := authz.ForServiceAccount(ctx, sa.UUID)
			if err != nil {
				return err
			}

			for _, r := range serviceAccountFromInput.Roles {
				if slices.Contains(deprecatedRoles, r.Name) {
					continue
				}

				if hasGlobalRoleRole(r.Name, existingRoles) {
					continue
				}

				if err := authz.AssignRoleToServiceAccount(ctx, sa.UUID, r.Name); err != nil {
					return err
				}
			}

			if err := serviceaccount.CreateToken(ctx, serviceAccountFromInput.APIKey, sa.UUID); err != nil {
				return err
			}
		}

		page, err := pagination.ParsePage(ptr.To(4000), nil, nil, nil)
		if err != nil {
			return err
		}

		// remove all NAIS service accounts that is not present in the JSON input
		all, err := serviceaccount.List(ctx, page)
		if err != nil {
			return err
		}

		for _, sa := range all.Nodes() {
			if !strings.HasPrefix(sa.Name, naisServiceAccountPrefix) {
				continue
			}

			if _, shouldExist := names[sa.Name]; shouldExist {
				continue
			}

			if err := serviceaccount.DeleteStatic(ctx, sa.UUID); err != nil {
				return err
			}
		}

		return nil
	})
}

func hasGlobalRoleRole(roleName string, existingRoles []*authz.Role) bool {
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
