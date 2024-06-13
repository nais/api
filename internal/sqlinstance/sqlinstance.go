package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/sqlinstance/fake"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type Client struct {
	metrics      *Metrics
	admin        *SqlAdminService
	informers    k8s.ClusterInformers
	log          logrus.FieldLogger
	fakesEnabled bool
}

type ClientOption func(*Client)

func WithFakeClients(enabled bool) ClientOption {
	return func(c *Client) {
		c.fakesEnabled = enabled
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

	metricsClientOps := make([]option.ClientOption, 0)
	sqladminClientOpts := make([]option.ClientOption, 0)
	if client.fakesEnabled {
		fakeGoogleApi, err := fake.NewFakeGoogleAPI(fake.WithInformerInstanceLister(informers))
		if err != nil {
			return nil, err
		}
		metricsClientOps = append(metricsClientOps, fakeGoogleApi.ClientGRPCOptions...)
		sqladminClientOpts = append(sqladminClientOpts, fakeGoogleApi.ClientHTTPOptions...)
	}

	metrics, err := NewMetrics(ctx, db, log, metricsClientOps...)
	if err != nil {
		return nil, err
	}
	client.metrics = metrics

	admin, err := NewSqlAdminService(ctx, log, sqladminClientOpts...)
	if err != nil {
		return nil, err
	}
	client.admin = admin

	return client, nil
}
