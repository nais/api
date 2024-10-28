package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"
)

type Client struct {
	Admin *SQLAdminService

	metrics         *Metrics
	log             logrus.FieldLogger
	fakesEnabled    bool
	instanceWatcher *watcher.Watcher[*SQLInstance]
}

type ClientOption func(*Client)

func WithFakeClients(enabled bool) ClientOption {
	return func(c *Client) {
		c.fakesEnabled = enabled
	}
}

func WithInstanceWatcher(w *watcher.Watcher[*SQLInstance]) ClientOption {
	return func(c *Client) {
		c.instanceWatcher = w
	}
}

func NewClient(ctx context.Context, log logrus.FieldLogger, opts ...ClientOption) (*Client, error) {
	client := &Client{
		log: log,
	}

	for _, opt := range opts {
		opt(client)
	}

	metricsClientOps := make([]option.ClientOption, 0)
	sqladminClientOpts := make([]option.ClientOption, 0)
	if client.fakesEnabled {
		fakeGoogleAPI, err := newFakeGoogleAPI(client.instanceWatcher)
		if err != nil {
			return nil, err
		}
		metricsClientOps = append(metricsClientOps, fakeGoogleAPI.ClientGRPCOptions...)
		sqladminClientOpts = append(sqladminClientOpts, fakeGoogleAPI.ClientHTTPOptions...)
	}

	metrics, err := NewMetrics(ctx, log, metricsClientOps...)
	if err != nil {
		return nil, err
	}
	client.metrics = metrics

	admin, err := NewSQLAdminService(ctx, log, sqladminClientOpts...)
	if err != nil {
		return nil, err
	}
	client.Admin = admin

	return client, nil
}
