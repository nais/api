package unleash

import (
	"context"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2/google"
	"time"
)

type Manager struct {
	clientMap clusterClients
}

func NewManager(tenant string, clusters []string, opts ...Opt) (*Manager, error) {
	clientMap, err := createClientMap(tenant, clusters, opts...)
	if err != nil {
		var authErr *google.AuthenticationError
		if errors.As(err, &authErr) {
			return nil, fmt.Errorf("unable to create k8s client. You should probably run `gcloud auth login --update-adc` and authenticate with your @nais.io-account before starting api: %w", err)
		}
		return nil, err
	}

	return &Manager{
		clientMap: clientMap,
	}, nil
}

func (m Manager) Start(ctx context.Context, log logrus.FieldLogger) error {
	for cluster, informers := range m.clientMap {
		log.WithField("cluster", cluster).Infof("starting informers")
		for _, informer := range informers.informers {
			go informer.Informer().Run(ctx.Done())
		}
	}

	for env, informers := range m.clientMap {
		for _, informer := range informers.informers {
			for !informer.Informer().HasSynced() {
				log.Infof("waiting for informer in %q to sync", env)

				select {
				case <-ctx.Done():
					return fmt.Errorf("informers not started: %w", ctx.Err())
				default:
					time.Sleep(2 * time.Second)
				}
			}
		}
	}
	return nil
}