package opensearch

import (
	"context"

	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/thirdparty/aivencache"
	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/vikstrous/dataloadgen"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

type AivenDataLoaderKey struct {
	Project     string
	ServiceName string
}

func NewLoaderContext(ctx context.Context, watcher *watcher.Watcher[*OpenSearch], aivenClient aivencache.AivenClient, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(watcher, aivenClient, logger))
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

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	client        *client
	watcher       *watcher.Watcher[*OpenSearch]
	versionLoader *dataloadgen.Loader[*AivenDataLoaderKey, string]
	aivenProjects map[string]struct {
		ID  string
		VPC string
	} // Maps our environment names to Aiven project names
	tenantName string
}

func newLoaders(watcher *watcher.Watcher[*OpenSearch], aivenClient aivencache.AivenClient, logger logrus.FieldLogger) *loaders {
	client := &client{
		watcher: watcher,
	}

	versionLoader := &dataloader{aivenClient: aivenClient, log: logger}

	return &loaders{
		client:        client,
		watcher:       watcher,
		versionLoader: dataloadgen.NewLoader(versionLoader.getVersions, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	aivenClient aivencache.AivenClient
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
