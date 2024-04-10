package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	Metrics   *Metrics
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
}

func NewClient(ctx context.Context, informers k8s.ClusterInformers, log logrus.FieldLogger) (*Client, error) {
	metrics, err := NewMetrics(ctx, log)
	if err != nil {
		return nil, err
	}
	return &Client{
		Metrics:   metrics,
		informers: informers,
		log:       log,
	}, nil
}

func (c *Client) WithMetrics(metrics *Metrics) *Client {
	c.Metrics = metrics
	return c
}
