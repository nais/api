package application

import (
	"context"

	"github.com/nais/api/internal/graph/model"
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
	k8sClient         *k8s.Client
	applicationLoader *dataloadgen.Loader[applicationIdentifier, *Application]
}

func newLoaders(k8sClient *k8s.Client, opts []dataloadgen.Option) *loaders {
	applicationLoader := &dataloader{
		k8sClient: k8sClient,
	}

	return &loaders{
		k8sClient:         k8sClient,
		applicationLoader: dataloadgen.NewLoader(applicationLoader.list, opts...),
	}
}

type dataloader struct {
	k8sClient *k8s.Client
}

func (l dataloader) getApplications(ctx context.Context, ids []applicationIdentifier) ([]*model.App, error) {
	return []*model.App{}, nil
}

type applicationIdentifier struct {
	namespace   string
	environment string
	name        string
}

func (l dataloader) list(ctx context.Context, ids []applicationIdentifier) ([]*Application, []error) {
	getID := func(obj *Application) applicationIdentifier {
		return applicationIdentifier{
			namespace:   obj.TeamSlug.String(),
			environment: obj.EnvironmentName,
			name:        obj.Name,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.getApplications, toGraphApplication, getID)
}
