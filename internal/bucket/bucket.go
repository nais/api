package bucket

import (
	gcpStorage "cloud.google.com/go/storage"
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	informers        k8s.ClusterInformers
	log              logrus.FieldLogger
	gcpStorageClient gcpStorage.Client
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, gcpStorageClient gcpStorage.Client) *Client {
	return &Client{
		informers:        informers,
		log:              log,
		gcpStorageClient: gcpStorageClient,
	}
}
