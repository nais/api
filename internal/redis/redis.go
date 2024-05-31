package redis

import (
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/k8s"
	"github.com/sirupsen/logrus"
)

type Client struct {
	informers k8s.ClusterInformers
	db        redisClientDatabase
	log       logrus.FieldLogger
}

type redisClientDatabase interface {
	database.CostRepo
	database.TeamRepo
}

func NewClient(informers k8s.ClusterInformers, log logrus.FieldLogger, db redisClientDatabase) *Client {
	return &Client{
		informers: informers,
		log:       log,
		db:        db,
	}
}
