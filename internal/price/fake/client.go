package fake

import (
	"context"

	"github.com/nais/api/internal/price"
)

type FakeClient struct{}

func NewClient() *FakeClient {
	return &FakeClient{}
}

func (s *FakeClient) GetUnitPrice(ctx context.Context, skuID string) (*price.Price, error) {
	return &price.Price{
		Value: 0.0042,
	}, nil
}
