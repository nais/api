package opensearch

import (
	"context"

	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/aiven"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/vikstrous/dataloadgen"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const loadersKey ctxKey = iota

var naisGVR = schema.GroupVersionResource{
	Group:    "nais.io",
	Version:  "v1",
	Resource: "opensearches",
}

type AivenDataLoaderKey struct {
	Project     string
	ServiceName string
}

func NewLoaderContext(ctx context.Context, tenantName string, openSearchWatcher, naisOpenSearchWatcher *watcher.Watcher[*OpenSearch], aivenClient aiven.AivenClient, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(tenantName, openSearchWatcher, naisOpenSearchWatcher, aivenClient, logger))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*OpenSearch] {
	w := watcher.Watch(mgr, &OpenSearch{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toOpenSearch(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "aiven.io",
		Version:  "v1alpha1",
		Resource: "opensearches",
	}))
	w.Start(ctx)
	return w
}

func NewNaisOpenSearchWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*OpenSearch] {
	w := watcher.Watch(mgr, &OpenSearch{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		v, err := kubernetes.ToConcrete[naiscrd.OpenSearch](o)
		if err != nil {
			return nil, false
		}
		ret, err := toOpenSearchFromNais(v, environmentName)
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
	client        *client
	watcher       *watcher.Watcher[*OpenSearch]
	naisWatcher   *watcher.Watcher[*OpenSearch]
	versionLoader *dataloadgen.Loader[*AivenDataLoaderKey, string]
	tenantName    string
	aivenClient   aiven.AivenClient
}

func newLoaders(tenantName string, watcher, naisOpenSearchWatcher *watcher.Watcher[*OpenSearch], aivenClient aiven.AivenClient, logger logrus.FieldLogger) *loaders {
	client := &client{}

	versionLoader := &dataloader{aivenClient: aivenClient, log: logger}

	return &loaders{
		client:        client,
		watcher:       watcher,
		naisWatcher:   naisOpenSearchWatcher,
		tenantName:    tenantName,
		versionLoader: dataloadgen.NewLoader(versionLoader.getVersions, loader.DefaultDataLoaderOptions...),
		aivenClient:   aivenClient,
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

type dataloader struct {
	aivenClient aiven.AivenClient
	log         logrus.FieldLogger
}

func (l dataloader) getVersions(ctx context.Context, aivenDataLoaderKeys []*AivenDataLoaderKey) ([]string, []error) {
	wg := pool.New().WithContext(ctx)
	rets := make([]string, len(aivenDataLoaderKeys))
	errs := make([]error, len(aivenDataLoaderKeys))

	for i, pair := range aivenDataLoaderKeys {
		wg.Go(func(ctx context.Context) error {
			res, err := l.aivenClient.ServiceGet(ctx, pair.Project, pair.ServiceName)
			if err != nil {
				errs[i] = err
			} else {
				if res.Metadata != nil {
					if version, ok := res.Metadata["opensearch_version"]; ok {
						rets[i] = version.(string)
					}
				}
			}
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		l.log.WithError(err).Error("error waiting for dataloader")
	}

	return rets, errs
}
