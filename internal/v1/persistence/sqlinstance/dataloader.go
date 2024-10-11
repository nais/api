package sqlinstance

import (
	"context"

	"github.com/nais/api/internal/sqlinstance"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, sqlAdminService *sqlinstance.SqlAdminService, sqlDatabaseWatcher *watcher.Watcher[*SQLDatabase], sqlInstanceWatcher *watcher.Watcher[*SQLInstance]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(sqlAdminService, sqlDatabaseWatcher, sqlInstanceWatcher))
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
	sqlAdminService    *sqlinstance.SqlAdminService
	sqlDatabaseWatcher *watcher.Watcher[*SQLDatabase]
	sqlInstanceWatcher *watcher.Watcher[*SQLInstance]
}

func newLoaders(sqlAdminService *sqlinstance.SqlAdminService, sqlDatabaseWatcher *watcher.Watcher[*SQLDatabase], sqlInstanceWatcher *watcher.Watcher[*SQLInstance]) *loaders {
	return &loaders{
		sqlAdminService:    sqlAdminService,
		sqlDatabaseWatcher: sqlDatabaseWatcher,
		sqlInstanceWatcher: sqlInstanceWatcher,
	}
}
