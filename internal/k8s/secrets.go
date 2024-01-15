package k8s

import (
	"context"
	"github.com/nais/api/internal/graph/model"
)

func (c *Client) GetSecret(ctx context.Context, name, team, env string) (*model.Secret, error) {
	return nil, nil
}

func (c *Client) CreateSecret(ctx context.Context, secret *model.Secret) error { return nil }

func (c *Client) UpdateSecret(ctx context.Context, secret *model.Secret) error { return nil }

func (c *Client) DeleteSecret(ctx context.Context, secret *model.Secret) error { return nil }
