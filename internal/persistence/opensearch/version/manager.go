package version

import (
	"context"

	"github.com/nais/api/internal/thirdparty/aivencache"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	aivenClient aivencache.AivenClient
	log         *logrus.Entry
}

func NewManager(_ context.Context, client aivencache.AivenClient, log *logrus.Entry) (*Manager, error) {
	return &Manager{
		aivenClient: client,
		log:         log,
	}, nil
}
