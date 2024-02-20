package database

import (
	"context"

	"github.com/nais/api/internal/database/gensql"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
)

type ServiceAccountRepo interface {
	CreateAPIKey(ctx context.Context, apiKey string, serviceAccountID uuid.UUID) error
	CreateServiceAccount(ctx context.Context, name string) (*ServiceAccount, error)
	DeleteServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error
	GetServiceAccountByApiKey(ctx context.Context, apiKey string) (*ServiceAccount, error)
	GetServiceAccountByName(ctx context.Context, name string) (*ServiceAccount, error)
	GetServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) ([]*authz.Role, error)
	GetServiceAccounts(ctx context.Context) ([]*ServiceAccount, error)
	GetServiceAccountsByIDs(ctx context.Context, ids []uuid.UUID) ([]*ServiceAccount, error)
	RemoveAllServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) error
	RemoveApiKeysFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error
}

var _ ServiceAccountRepo = (*database)(nil)

type ServiceAccount struct {
	*gensql.ServiceAccount
}

func (s ServiceAccount) GetID() uuid.UUID {
	return s.ID
}

func (s ServiceAccount) Identity() string {
	return s.Name
}

func (s ServiceAccount) IsServiceAccount() bool {
	return true
}

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

	serviceAccounts := make([]*ServiceAccount, len(rows))
	for i, row := range rows {
		serviceAccounts[i] = &ServiceAccount{ServiceAccount: row}
	}

	return serviceAccounts, nil
}

func (d *database) GetServiceAccountsByIDs(ctx context.Context, ids []uuid.UUID) ([]*ServiceAccount, error) {
	rows, err := d.querier.GetServiceAccountsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	serviceAccounts := make([]*ServiceAccount, len(rows))
	for i, row := range rows {
		serviceAccounts[i] = &ServiceAccount{ServiceAccount: row}
	}

	return serviceAccounts, nil
}

func (d *database) DeleteServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error {
	return d.querier.DeleteServiceAccount(ctx, serviceAccountID)
}

func (d *database) RemoveAllServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) error {
	return d.querier.RemoveAllServiceAccountRoles(ctx, serviceAccountID)
}

func (d *database) GetServiceAccountRoles(ctx context.Context, serviceAccountID uuid.UUID) ([]*authz.Role, error) {
	serviceAccountRoles, err := d.querier.GetServiceAccountRoles(ctx, serviceAccountID)
	if err != nil {
		return nil, err
	}

	roles := make([]*authz.Role, len(serviceAccountRoles))
	for i, serviceAccountRole := range serviceAccountRoles {
		role, err := d.roleFromRoleBinding(ctx, serviceAccountRole.RoleName, serviceAccountRole.TargetServiceAccountID, serviceAccountRole.TargetTeamSlug)
		if err != nil {
			return nil, err
		}
		roles[i] = role
	}

	return roles, nil
}

func (d *database) CreateAPIKey(ctx context.Context, apiKey string, serviceAccountID uuid.UUID) error {
	return d.querier.CreateAPIKey(ctx, gensql.CreateAPIKeyParams{
		ApiKey:           apiKey,
		ServiceAccountID: serviceAccountID,
	})
}

func (d *database) RemoveApiKeysFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error {
	return d.querier.RemoveApiKeysFromServiceAccount(ctx, serviceAccountID)
}
