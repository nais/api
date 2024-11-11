package secret

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/team"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ctxKey int

const loadersKey ctxKey = iota

type ClientCreator func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error)

func NewLoaderContext(ctx context.Context, clientCreator ClientCreator, environments []string, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		clientCreator: clientCreator,
		log:           log,
		environments:  environments,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	clientCreator ClientCreator
	log           logrus.FieldLogger
	environments  []string
}

func (l *loaders) Client(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
	return l.clientCreator(ctx, environment)
}

func (l *loaders) Clients(ctx context.Context) (map[string]dynamic.NamespaceableResourceInterface, error) {
	clients := make(map[string]dynamic.NamespaceableResourceInterface)

	for _, environment := range l.environments {
		client, err := l.Client(ctx, environment)
		if err != nil {
			return nil, fmt.Errorf("creating client for environment %q: %w", environment, err)
		}

		clients[environment] = client
	}

	return clients, nil
}

func CreatorFromConfig(ctx context.Context, configs map[string]*rest.Config) ClientCreator {
	return func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
		config, exists := configs[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}

		actor := authz.ActorFromContext(ctx)

		groups, err := team.ListGCPGroupsForUser(ctx, actor.User.GetID())
		if err != nil {
			return nil, fmt.Errorf("listing GCP groups for user: %w", err)
		}

		cfg := rest.CopyConfig(config)
		cfg.Impersonate = rest.ImpersonationConfig{
			UserName: actor.User.Identity(),
			Groups:   groups,
		}

		client, err := dynamic.NewForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("creating dynamic client: %w", err)
		}

		return client.Resource(v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets))), nil
	}
}

func CreatorFromClients(clients map[string]dynamic.Interface) ClientCreator {
	return func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
		client, exists := clients[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}

		return client.Resource(v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets))), nil
	}
}
