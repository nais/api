package opensearch

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
	k8sClient        *client
	openSearchLoader *dataloadgen.Loader[resourceIdentifier, *OpenSearch]
}

func newLoaders(k8sClient *k8s.Client, opts []dataloadgen.Option) *loaders {
	client := &client{
		informers: k8sClient.Informers(),
	}

	datasetLoader := &dataloader{
		k8sClient: client,
	}

	return &loaders{
		k8sClient:        client,
		openSearchLoader: dataloadgen.NewLoader(datasetLoader.list, opts...),
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

func (l dataloader) list(ctx context.Context, ids []resourceIdentifier) ([]*OpenSearch, []error) {
	makeKey := func(obj *OpenSearch) resourceIdentifier {
		return resourceIdentifier{
			namespace:   obj.TeamSlug.String(),
			environment: obj.EnvironmentName,
			name:        obj.Name,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.k8sClient.getOpenSearches, func(o *OpenSearch) *OpenSearch { return o }, makeKey)
}