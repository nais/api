package maintenance

import (
	"context"

	"github.com/aiven/go-client-codegen"
	"github.com/sirupsen/logrus"
)

type Manager struct {
	client aiven.Client
	log    *logrus.Entry
}

func NewManager(ctx context.Context, token string, log *logrus.Entry) (*Manager, error) {
	client, err := aiven.NewClient(aiven.TokenOpt(token), aiven.UserAgentOpt("nais-api"))
	if err != nil {
		return nil, err
	}

	return &Manager{
		client: client,
		log:    log,
	}, nil
}
