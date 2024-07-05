package bigquery

import (
	"context"

	"github.com/nais/api/internal/graphv1/loaderv1"
	"github.com/nais/api/internal/k8s"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, k8sClient *k8s.Client, defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(k8sClient, defaultOpts))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	k8sClient     *client
	datasetLoader *dataloadgen.Loader[resourceIdentifier, *BigQueryDataset]
}

func newLoaders(k8sClient *k8s.Client, opts []dataloadgen.Option) *loaders {
	client := &client{
		informers: k8sClient.Informers(),
	}

	datasetLoader := &dataloader{
		k8sClient: client,
	}

	return &loaders{
		k8sClient:     client,
		datasetLoader: dataloadgen.NewLoader(datasetLoader.list, opts...),
	}
}

type dataloader struct {
	k8sClient *client
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
	return loaderv1.LoadModels(ctx, ids, l.k8sClient.getBigQueryDatasets, func(d *BigQueryDataset) *BigQueryDataset { return d }, makeKey)
}
