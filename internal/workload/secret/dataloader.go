package secret

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ctxKey int

const loadersKey ctxKey = iota

// ClientCreator creates a client that impersonates the user (for read operations requiring elevation)
type ClientCreator func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error)

// ServiceAccountClientCreator creates a client using the API's service account (for write operations)
type ServiceAccountClientCreator func(environment string) (dynamic.NamespaceableResourceInterface, error)

func NewLoaderContext(ctx context.Context, clientCreator ClientCreator, saClientCreator ServiceAccountClientCreator, environments []string, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		clientCreator:   clientCreator,
		saClientCreator: saClientCreator,
		log:             log,
		environments:    environments,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	clientCreator   ClientCreator
	saClientCreator ServiceAccountClientCreator
	log             logrus.FieldLogger
	environments    []string
}

// Client returns an impersonated client for read operations (requires user to have elevation)
func (l *loaders) Client(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
	return l.clientCreator(ctx, environment)
}

// ServiceAccountClient returns a client using API's service account for write operations
func (l *loaders) ServiceAccountClient(environment string) (dynamic.NamespaceableResourceInterface, error) {
	return l.saClientCreator(environment)
}

// Clients returns impersonated clients for all environments
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

// ServiceAccountClients returns service account clients for all environments
func (l *loaders) ServiceAccountClients() (map[string]dynamic.NamespaceableResourceInterface, error) {
	clients := make(map[string]dynamic.NamespaceableResourceInterface)

	for _, environment := range l.environments {
		client, err := l.ServiceAccountClient(environment)
		if err != nil {
			return nil, fmt.Errorf("creating SA client for environment %q: %w", environment, err)
		}

		clients[environment] = client
	}

	return clients, nil
}

// CreatorFromConfig creates an impersonated client creator (for read operations)
func CreatorFromConfig(ctx context.Context, configs map[string]*rest.Config) ClientCreator {
	return func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
		config, exists := configs[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}

		actor := authz.ActorFromContext(ctx)

		groups, err := user.ListGCPGroupsForUser(ctx, actor.User.GetID())
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

// ServiceAccountCreatorFromConfig creates a service account client creator (for write operations)
func ServiceAccountCreatorFromConfig(configs map[string]*rest.Config) ServiceAccountClientCreator {
	// Pre-create clients for each environment
	clients := make(map[string]dynamic.NamespaceableResourceInterface)

	return func(environment string) (dynamic.NamespaceableResourceInterface, error) {
		// Return cached client if exists
		if client, exists := clients[environment]; exists {
			return client, nil
		}

		config, exists := configs[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}

		// No impersonation - use API's service account
		client, err := dynamic.NewForConfig(config)
		if err != nil {
			return nil, fmt.Errorf("creating dynamic client: %w", err)
		}

		resourceClient := client.Resource(v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets)))
		clients[environment] = resourceClient
		return resourceClient, nil
	}
}

// CreatorFromClients creates client creators from pre-existing dynamic clients (for testing/fake mode)
func CreatorFromClients(clients map[string]dynamic.Interface) (ClientCreator, ServiceAccountClientCreator) {
	clientCreator := func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
		client, exists := clients[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}
		return client.Resource(v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets))), nil
	}

	saClientCreator := func(environment string) (dynamic.NamespaceableResourceInterface, error) {
		client, exists := clients[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}
		return client.Resource(v1.SchemeGroupVersion.WithResource(string(v1.ResourceSecrets))), nil
	}

	return clientCreator, saClientCreator
}
