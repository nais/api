package redis

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
	metrics   Metrics
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, costRepo database.CostRepo) *Client {
	return &Client{
		informers: informers,
		log:       log,
		metrics:   Metrics{log: log, costRepo: costRepo},
	}
}
