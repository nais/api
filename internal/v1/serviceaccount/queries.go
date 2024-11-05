package serviceaccount

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/v1/serviceaccount/serviceaccountsql"
)

func GetByApiKey(ctx context.Context, apiKey string) (*ServiceAccount, error) {
	sa, err := db(ctx).GetByApiKey(ctx, apiKey)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{
		UUID: sa.ID,
		Name: sa.Name,
	}, nil
}

func GetByName(ctx context.Context, name string) (*ServiceAccount, error) {
	sa, err := db(ctx).GetByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{
		UUID: sa.ID,
		Name: sa.Name,
	}, nil
}

func Create(ctx context.Context, name string) (*ServiceAccount, error) {
	sa, err := db(ctx).Create(ctx, name)
	if err != nil {
		return nil, err
	}

	return &ServiceAccount{
		UUID: sa.ID,
		Name: sa.Name,
	}, nil
}

func RemoveApiKeysFromServiceAccount(ctx context.Context, serviceAccountID uuid.UUID) error {
	return db(ctx).RemoveApiKeysFromServiceAccount(ctx, serviceAccountID)
}

func CreateAPIKey(ctx context.Context, apiKey string, serviceAccountID uuid.UUID) error {
	return db(ctx).CreateAPIKey(ctx, serviceaccountsql.CreateAPIKeyParams{
		ApiKey:           apiKey,
		ServiceAccountID: serviceAccountID,
	})
}

func List(ctx context.Context) ([]*ServiceAccount, error) {
	rows, err := db(ctx).List(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*ServiceAccount, len(rows))
	for i, row := range rows {
		ret[i] = &ServiceAccount{
			UUID: row.ID,
			Name: row.Name,
		}
	}

	return ret, nil
}

func Delete(ctx context.Context, id uuid.UUID) error {
	return db(ctx).Delete(ctx, id)
}
