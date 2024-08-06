package application

import (
	"context"
	"time"

	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, mgr *watcher.Manager, defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(mgr, defaultOpts))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	appWatcher        *watcher.Watcher[*nais_io_v1alpha1.Application]
	applicationLoader *dataloadgen.Loader[applicationIdentifier, *Application]
}

func newLoaders(mgr *watcher.Manager, opts []dataloadgen.Option) *loaders {
	appWatcher := watcher.Watch(mgr, &nais_io_v1alpha1.Application{})
	if !appWatcher.WaitForReady(context.Background(), 10*time.Second) {
		panic("failed to wait for appWatcher to become ready")
	}
	applicationLoader := &dataloader{
		mgr: appWatcher,
	}

	return &loaders{
		appWatcher:        appWatcher,
		applicationLoader: dataloadgen.NewLoader(applicationLoader.list, opts...),
	}
}

type dataloader struct {
	mgr *watcher.Watcher[*nais_io_v1alpha1.Application]
}

func (l dataloader) getApplications(ctx context.Context, ids []applicationIdentifier) ([]*nais_io_v1alpha1.Application, error) {
	ret := make([]*nais_io_v1alpha1.Application, 0)
	// for _, id := range ids {
	// 	app, err := l.mgr.App(ctx, id.name, id.namespace, id.environment)
	// 	if err != nil {
	// 		fmt.Println("error fetching application", err)
	// 		continue
	// 	}
	// 	ret = append(ret, app)
	// }
	return ret, nil
}

type applicationIdentifier struct {
	namespace   string
	environment string
	name        string
}

func (l dataloader) list(ctx context.Context, ids []applicationIdentifier) ([]*Application, []error) {
	makeKey := func(obj *Application) applicationIdentifier {
		return applicationIdentifier{
			namespace:   obj.TeamSlug.String(),
			environment: obj.EnvironmentName,
			name:        obj.Name,
		}
	}
	return loaderv1.LoadModels(ctx, ids, l.getApplications, toGraphApplication, makeKey)
}
