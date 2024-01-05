package db

import (
	"context"

	"github.com/nais/api/internal/sqlc"

	"github.com/google/uuid"
)

func (d *database) CreateServiceAccount(ctx context.Context, name string) (*ServiceAccount, error) {
	serviceAccount, err := d.querier.CreateServiceAccount(ctx, name)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{ServiceAccount: serviceAccount}, nil
}

func (d *database) GetServiceAccountByName(ctx context.Context, name string) (*ServiceAccount, error) {
	serviceAccount, err := d.querier.GetServiceAccountByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{ServiceAccount: serviceAccount}, nil
}

func (d *database) GetServiceAccountByApiKey(ctx context.Context, apiKey string) (*ServiceAccount, error) {
	serviceAccount, err := d.querier.GetServiceAccountByApiKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{ServiceAccount: serviceAccount}, nil
}

func (d *database) GetServiceAccounts(ctx context.Context) ([]*ServiceAccount, error) {
	rows, err := d.querier.GetServiceAccounts(ctx)
	if err != nil {
		return nil, err
	}

	serviceAccounts := make([]*ServiceAccount, 0)
	for _, row := range rows {
		serviceAccounts = append(serviceAccounts, &ServiceAccount{ServiceAccount: row})
	}

	return serviceAccounts, nil
}

func (d *database) DeleteServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error {
	return d.querier.DeleteServiceAccount(ctx, serviceAccountID)
}

func (d *database) RemoveAllServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) error {
	return d.querier.RemoveAllServiceAccountRoles(ctx, serviceAccountID)
}

func (d *database) GetServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) ([]*Role, error) {
	serviceAccountRoles, err := d.querier.GetServiceAccountRoles(ctx, serviceAccountID)
	if err != nil {
		return nil, err
	}

	roles := make([]*Role, 0, len(serviceAccountRoles))
	for _, serviceAccountRole := range serviceAccountRoles {
		role, err := d.roleFromRoleBinding(ctx, serviceAccountRole.RoleName, serviceAccountRole.TargetServiceAccountID, serviceAccountRole.TargetTeamSlug)
		if err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}

	return roles, nil
}

func (d *database) CreateAPIKey(ctx context.Context, apiKey string, serviceAccountID uuid.UUID) error {
	return d.querier.CreateAPIKey(ctx, sqlc.CreateAPIKeyParams{
		ApiKey:           apiKey,
		ServiceAccountID: serviceAccountID,
	})
}

func (d *database) RemoveApiKeysFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error {
	return d.querier.RemoveApiKeysFromServiceAccount(ctx, serviceAccountID)
}
