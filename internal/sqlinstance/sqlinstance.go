package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/database"

	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	metrics   *Metrics
	admin     *SqlAdminService
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
}

type ClientOption func(*Client)

func WithMetrics(metrics *Metrics) ClientOption {
	return func(c *Client) {
		c.metrics = metrics
	}
}

func WithAdmin(admin *SqlAdminService) ClientOption {
	return func(c *Client) {
		c.admin = admin
	}
}

func NewClient(ctx context.Context, db database.Database, informers k8s.ClusterInformers, log logrus.FieldLogger, opts ...ClientOption) (*Client, error) {
	client := &Client{
		informers: informers,
		log:       log,
	}

	for _, opt := range opts {
		opt(client)
	}

	if client.metrics == nil {
		metrics, err := NewMetrics(ctx, db, log)
		if err != nil {
			return nil, err
		}
		client.metrics = metrics
	}

	if client.admin == nil {
		admin, err := NewSqlAdminService(ctx, log)
		if err != nil {
			return nil, err
		}
		client.admin = admin
	}

	return client, nil
}
