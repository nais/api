package bucket

import (
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger) *Client {
	return &Client{
		informers: informers,
		log:       log,
	}
}
