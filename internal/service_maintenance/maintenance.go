package servicemaintenance

import (
	"context"

	aiven "github.com/aiven/go-client-codegen"
	aivenservice "github.com/aiven/go-client-codegen/handler/service"
	"github.com/nais/api/internal/service_maintenance/fake"
	"github.com/sirupsen/logrus"
)

const (
	fakeAivenToken = "fake-aiven-token"
)

type Client interface {
	ServiceGet(context.Context, string, string, ...[2]string) (*aivenservice.ServiceGetOut, error)
}

type Manager struct {
	client Client
	log    *logrus.Entry
}

func NewManager(ctx context.Context, token string, log *logrus.Entry) (*Manager, error) {
	if token == fakeAivenToken {
		return NewFakeManager(ctx, log)
	}

	client, err := aiven.NewClient(aiven.TokenOpt(token), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		return nil, err
	}

	return &Manager{
		client: client,
		log:    log,
	}, nil
}

func NewFakeManager(_ context.Context, log *logrus.Entry) (*Manager, error) {
	return &Manager{
		client: fake.NewFakeAivenClient(),
		log:    log,
	}, nil
}
