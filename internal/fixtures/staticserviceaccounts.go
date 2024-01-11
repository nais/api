package fixtures

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nais/api/internal/auth/authz"
	db "github.com/nais/api/internal/database"
	sqlc "github.com/nais/api/internal/database/gensql"
)

type ServiceAccount struct {
	Name   string               `json:"name"`
	Roles  []ServiceAccountRole `json:"roles"`
	APIKey string               `json:"apiKey"`
}

type ServiceAccountRole struct {
	Name sqlc.RoleName `json:"name"`
}

const NaisServiceAccountPrefix = "nais-"

type ServiceAccounts []ServiceAccount

func (s *ServiceAccounts) UnmarshalJSON(value []byte) error {
	if len(value) == 0 {
		return nil
	}

	serviceAccounts := make([]ServiceAccount, 0)
	err := json.NewDecoder(bytes.NewReader(value)).Decode(&serviceAccounts)
	if err != nil {
		return err
	}

	for _, serviceAccount := range serviceAccounts {
		if !strings.HasPrefix(serviceAccount.Name, NaisServiceAccountPrefix) {
			return fmt.Errorf("service account is missing required %q prefix: %q", NaisServiceAccountPrefix, serviceAccount.Name)
		}

		if len(serviceAccount.Roles) == 0 {
			return fmt.Errorf("service account must have at least one role: %q", serviceAccount.Name)
		}

		if serviceAccount.APIKey == "" {
			return fmt.Errorf("service account is missing an API key: %q", serviceAccount.Name)
		}

		for _, role := range serviceAccount.Roles {
			if !role.Name.Valid() {
				return fmt.Errorf("invalid role name: %q for service account %q", role.Name, serviceAccount.Name)
			}
		}
	}

	*s = serviceAccounts
	return nil
}

// SetupStaticServiceAccounts Create a set of service accounts with roles and API keys
func SetupStaticServiceAccounts(ctx context.Context, database db.Database, serviceAccounts ServiceAccounts) error {
	return database.Transaction(ctx, func(ctx context.Context, dbtx db.Database) error {
		serviceAccountNames := make(map[string]struct{})
		for _, serviceAccountFromInput := range serviceAccounts {
			serviceAccountNames[serviceAccountFromInput.Name] = struct{}{}
			serviceAccount, err := dbtx.GetServiceAccountByName(ctx, serviceAccountFromInput.Name)
			if err != nil {
				serviceAccount, err = dbtx.CreateServiceAccount(ctx, serviceAccountFromInput.Name)
				if err != nil {
					return err
				}
			}

			err = dbtx.RemoveApiKeysFromServiceAccount(ctx, serviceAccount.ID)
			if err != nil {
				return err
			}

			existingRoles, err := dbtx.GetServiceAccountRoles(ctx, serviceAccount.ID)
			if err != nil {
				return err
			}

			for _, role := range serviceAccountFromInput.Roles {
				if hasGlobalRoleRole(role.Name, existingRoles) {
					continue
				}

				err = dbtx.AssignGlobalRoleToServiceAccount(ctx, serviceAccount.ID, role.Name)
				if err != nil {
					return err
				}
			}

			err = dbtx.CreateAPIKey(ctx, serviceAccountFromInput.APIKey, serviceAccount.ID)
			if err != nil {
				return err
			}
		}

		// remove all NAIS service accounts that is not present in the JSON input
		serviceAccounts, err := dbtx.GetServiceAccounts(ctx)
		if err != nil {
			return err
		}

		for _, serviceAccount := range serviceAccounts {
			if !strings.HasPrefix(serviceAccount.Name, NaisServiceAccountPrefix) {
				continue
			}

			if _, shouldExist := serviceAccountNames[serviceAccount.Name]; shouldExist {
				continue
			}

			if err := dbtx.DeleteServiceAccount(ctx, serviceAccount.ID); err != nil {
				return err
			}
		}

		return nil
	})
}

func hasGlobalRoleRole(roleName sqlc.RoleName, existingRoles []*authz.Role) bool {
	for _, r := range existingRoles {
		if r.TargetTeamSlug != nil {
			continue
		}

		if roleName == r.RoleName {
			return true
		}
	}

	return false
}
