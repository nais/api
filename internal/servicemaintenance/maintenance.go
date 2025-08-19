package servicemaintenance

import (
	"context"

	"github.com/nais/api/internal/thirdparty/aiven"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	aivenClient aiven.AivenClient
	log         *logrus.Entry
}

func NewManager(_ context.Context, client aiven.AivenClient, log *logrus.Entry) (*Manager, error) {
	return &Manager{
		aivenClient: client,
		log:         log,
	}, nil
}
