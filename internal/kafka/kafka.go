package kafka

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	informers k8s.ClusterInformers
	log       logrus.FieldLogger
	db        database.TeamRepo
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, db database.TeamRepo) *Client {
	return &Client{
		informers: informers,
		log:       log,
		db:        db,
	}
}
