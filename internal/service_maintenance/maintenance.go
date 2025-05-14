package servicemaintenance

import (
	"context"

	aivenservice "github.com/aiven/go-client-codegen/handler/service"
	"github.com/nais/api/internal/service_maintenance/fake"
	"github.com/sirupsen/logrus"
)

type AivenClient interface {
	ServiceGet(context.Context, string, string, ...[2]string) (*aivenservice.ServiceGetOut, error)
	ServiceMaintenanceStart(context.Context, string, string) error
}

type Manager struct {
	aivenClient AivenClient
	log         *logrus.Entry
}

func NewManager(_ context.Context, client AivenClient, log *logrus.Entry) (*Manager, error) {
	return &Manager{
		aivenClient: client,
		log:         log,
	}, nil
}

func NewFakeAivenClient() AivenClient {
	return fake.NewFakeAivenClient()
}
