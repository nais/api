package bigquery

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/vikstrous/dataloadgen"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, bqWatcher *watcher.Watcher[*BigQueryDataset], defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(bqWatcher, defaultOpts))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*BigQueryDataset] {
	w := watcher.Watch(mgr, &BigQueryDataset{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toBigQueryDataset(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "google.nais.io",
		Version:  "v1",
		Resource: "bigquerydatasets",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	watcher       *watcher.Watcher[*BigQueryDataset]
	datasetLoader *dataloadgen.Loader[resourceIdentifier, *BigQueryDataset]
}

func newLoaders(bqWatcher *watcher.Watcher[*BigQueryDataset], opts []dataloadgen.Option) *loaders {
	client := &client{
		watcher: bqWatcher,
	}

	datasetLoader := &dataloader{
		watcher: bqWatcher,
		client:  client,
	}

	return &loaders{
		watcher:       bqWatcher,
		datasetLoader: dataloadgen.NewLoader(datasetLoader.list, opts...),
	}
}

type dataloader struct {
	watcher *watcher.Watcher[*BigQueryDataset]
	client  *client
}

type resourceIdentifier struct {
	namespace   string
	environment string
	name        string
}

func (l dataloader) list(ctx context.Context, ids []resourceIdentifier) ([]*BigQueryDataset, []error) {
	makeKey := func(obj *BigQueryDataset) resourceIdentifier {
		return resourceIdentifier{
			namespace:   obj.TeamSlug.String(),
			environment: obj.EnvironmentName,
			name:        obj.Name,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.client.getBigQueryDatasets, func(d *BigQueryDataset) *BigQueryDataset { return d }, makeKey)
}
