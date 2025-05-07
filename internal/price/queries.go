package price

import "context"

func GetPrice(ctx context.Context, resourceType ResourceType) (*Price, error) {
	p, err := fromContext(ctx).client.Price.GetUnitPrice(ctx, resourceType)
	if err != nil {
		return nil, err
	}

	return p, nil
}
