package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/sourcegraph/conc/pool"
	"github.com/vikstrous/dataloadgen"
	"google.golang.org/api/sqladmin/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(
	ctx context.Context,
	client *Client,
	sqlDatabaseWatcher *watcher.Watcher[*SQLDatabase],
	sqlInstanceWatcher *watcher.Watcher[*SQLInstance],
	defaultOpts []dataloadgen.Option,
) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(client, sqlDatabaseWatcher, sqlInstanceWatcher, defaultOpts))
}

func NewInstanceWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*SQLInstance] {
	w := watcher.Watch(mgr, &SQLInstance{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toSQLInstance(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "sql.cnrm.cloud.google.com",
		Version:  "v1beta1",
		Resource: "sqlinstances",
	}))
	w.Start(ctx)
	return w
}

func NewDatabaseWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*SQLDatabase] {
	w := watcher.Watch(mgr, &SQLDatabase{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toSQLDatabase(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "sql.cnrm.cloud.google.com",
		Version:  "v1beta1",
		Resource: "sqldatabases",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	sqlAdminService    *SQLAdminService
	sqlMetricsService  *Metrics
	sqlDatabaseWatcher *watcher.Watcher[*SQLDatabase]
	sqlInstanceWatcher *watcher.Watcher[*SQLInstance]
	remoteSQLInstance  *dataloadgen.Loader[instanceKey, *sqladmin.DatabaseInstance]
}

func newLoaders(
	client *Client,
	sqlDatabaseWatcher *watcher.Watcher[*SQLDatabase],
	sqlInstanceWatcher *watcher.Watcher[*SQLInstance],
	defaultOpts []dataloadgen.Option,
) *loaders {
	dataloader := dataloader{sqlAdminService: client.Admin}
	return &loaders{
		sqlAdminService:    client.Admin,
		sqlMetricsService:  client.metrics,
		sqlDatabaseWatcher: sqlDatabaseWatcher,
		sqlInstanceWatcher: sqlInstanceWatcher,
		remoteSQLInstance:  dataloadgen.NewLoader(dataloader.remoteInstance, defaultOpts...),
	}
}

type instanceKey struct {
	projectID string
	name      string
}

type dataloader struct {
	sqlAdminService *SQLAdminService
}

func (l dataloader) remoteInstance(ctx context.Context, keys []instanceKey) ([]*sqladmin.DatabaseInstance, []error) {
	makeKey := func(obj *sqladmin.DatabaseInstance) instanceKey {
		return instanceKey{projectID: obj.Project, name: obj.Name}
	}

	type result struct {
		instance *sqladmin.DatabaseInstance
		err      error
	}
	wg := pool.NewWithResults[result]().WithMaxGoroutines(10)
	for _, key := range keys {
		wg.Go(func() result {
			i, err := l.sqlAdminService.GetInstance(ctx, key.projectID, key.name)
			return result{instance: i, err: err}
		})
	}

	res := wg.Wait()
	return loaderv1.LoadModelsWithError(ctx, keys, func(ctx context.Context, k []instanceKey) ([]result, error) {
		return res, nil
	}, func(i result) (*sqladmin.DatabaseInstance, error) {
		return i.instance, i.err
	}, makeKey)
}
