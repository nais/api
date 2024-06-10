package opensearch

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
	db        openSearchClientDatabase
}

type openSearchClientDatabase interface {
	database.CostRepo
	database.TeamRepo
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, db openSearchClientDatabase) *Client {
	return &Client{
		informers: informers,
		log:       log,
		db:        db,
	}
}
