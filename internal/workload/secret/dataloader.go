package secret

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type ctxKey int

const loadersKey ctxKey = iota

// ClientCreator creates a client that impersonates the user (for read operations requiring elevation)
type ClientCreator func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error)

func NewLoaderContext(ctx context.Context, watcher *watcher.Watcher[*Secret], clientCreator ClientCreator, environments []string, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		watcher:       watcher,
		clientCreator: clientCreator,
		log:           log,
		environments:  environments,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Secret] {
	w := watcher.Watch(
		mgr,
		&Secret{},
		watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (any, bool) {
			return toGraphSecret(o, environmentName)
		}),
		watcher.WithTransformer(transformSecret),
		watcher.WithInformerFilter(kubernetes.IsManagedByConsoleLabelSelector()),
		watcher.WithGVR(corev1.SchemeGroupVersion.WithResource("secrets")),
	)
	w.Start(ctx)
	return w
}

// transformSecret removes secret values from the object before caching,
// keeping only the key names. This ensures secret values are never stored in memory.
func transformSecret(in any) (any, error) {
	secret, ok := in.(*unstructured.Unstructured)
	if !ok {
		return nil, fmt.Errorf("expected *unstructured.Unstructured, got %T", in)
	}

	// Extract key names from data before removing it
	data, _, _ := unstructured.NestedMap(secret.Object, "data")
	keys := slices.Sorted(maps.Keys(data))

	// Remove the actual secret data
	unstructured.RemoveNestedField(secret.Object, "data")
	unstructured.RemoveNestedField(secret.Object, "stringData")

	// Store key names in an annotation for later retrieval
	annotations := secret.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Store keys as comma-separated list (empty string if no keys)
	keyList := ""
	for i, k := range keys {
		if i > 0 {
			keyList += ","
		}
		keyList += k
	}
	annotations[annotationSecretKeys] = keyList
	secret.SetAnnotations(annotations)

	// Remove other unnecessary fields to reduce memory usage
	unstructured.RemoveNestedField(secret.Object, "metadata", "managedFields")

	return secret, nil
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	watcher       *watcher.Watcher[*Secret]
	clientCreator ClientCreator
	log           logrus.FieldLogger
	environments  []string
}

// Watcher returns the secret watcher
func (l *loaders) Watcher() *watcher.Watcher[*Secret] {
	return l.watcher
}

// Client returns an impersonated client for read operations (requires user to have elevation)
func (l *loaders) Client(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
	return l.clientCreator(ctx, environment)
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

		return client.Resource(corev1.SchemeGroupVersion.WithResource(string(corev1.ResourceSecrets))), nil
	}
}

// CreatorFromClients creates a client creator from pre-existing dynamic clients (for testing/fake mode)
func CreatorFromClients(clients map[string]dynamic.Interface) ClientCreator {
	return func(ctx context.Context, environment string) (dynamic.NamespaceableResourceInterface, error) {
		client, exists := clients[environment]
		if !exists {
			return nil, apierror.Errorf("Environment %q does not exist.", environment)
		}
		return client.Resource(corev1.SchemeGroupVersion.WithResource(string(corev1.ResourceSecrets))), nil
	}
}
