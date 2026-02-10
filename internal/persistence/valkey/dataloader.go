package valkey

import (
	"context"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/aiven"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const loadersKey ctxKey = iota

var naisGVR = schema.GroupVersionResource{
	Group:    "nais.io",
	Version:  "v1",
	Resource: "valkeys",
}

func NewLoaderContext(ctx context.Context, tenantName string, valkeyWatcher, naisValkeyWatcher *watcher.Watcher[*Valkey], aivenClient aiven.AivenClient) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(tenantName, valkeyWatcher, naisValkeyWatcher, aivenClient))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Valkey] {
	w := watcher.Watch(mgr, &Valkey{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toValkey(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "aiven.io",
		Version:  "v1alpha1",
		Resource: "valkeys",
	}))
	w.Start(ctx)
	return w
}

func NewNaisValkeyWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*Valkey] {
	w := watcher.Watch(mgr, &Valkey{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		v, err := kubernetes.ToConcrete[naiscrd.Valkey](o)
		if err != nil {
			return nil, false
		}
		ret, err := toValkeyFromNais(v, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(naisGVR))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client      *client
	tenantName  string
	watcher     *watcher.Watcher[*Valkey]
	naisWatcher *watcher.Watcher[*Valkey]
	aivenClient aiven.AivenClient
}

func newLoaders(tenantName string, watcher, naisValkeyWatcher *watcher.Watcher[*Valkey], aivenClient aiven.AivenClient) *loaders {
	client := &client{}

	return &loaders{
		client:      client,
		tenantName:  tenantName,
		watcher:     watcher,
		naisWatcher: naisValkeyWatcher,
		aivenClient: aivenClient,
	}
}

func newK8sClient(ctx context.Context, environmentName string, teamSlug slug.Slug) (dynamic.ResourceInterface, error) {
	sysClient, err := fromContext(ctx).watcher.ImpersonatedClient(
		ctx,
		environmentName,
		watcher.WithImpersonatedClientGVR(naisGVR),
	)
	if err != nil {
		return nil, err
	}
	return sysClient.Namespace(teamSlug.String()), nil
}
