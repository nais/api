package bucket

import (
	"context"

	"github.com/nais/api/internal/database"

	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Bucket struct {
	Name string
	Size string
	Cost string
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
