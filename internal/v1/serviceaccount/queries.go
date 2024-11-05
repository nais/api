package serviceaccount

import (
	"context"
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
