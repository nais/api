package postgres

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(
	ctx context.Context,
	postgresWatcher *watcher.Watcher[*PostgresInstance],
	auditLogProjectID string,
	auditLogLocation string,
) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(postgresWatcher, auditLogProjectID, auditLogLocation))
}

type loaders struct {
	postgresWatcher   *watcher.Watcher[*PostgresInstance]
	auditLogProjectID string
	auditLogLocation  string
}

func newLoaders(
	postgresWatcher *watcher.Watcher[*PostgresInstance],
	auditLogProjectID string,
	auditLogLocation string,
) *loaders {
	return &loaders{
		postgresWatcher:   postgresWatcher,
		auditLogProjectID: auditLogProjectID,
		auditLogLocation:  auditLogLocation,
	}
}

// GetAuditLogConfig returns the audit log configuration from context
func GetAuditLogConfig(ctx context.Context) (projectID, location string) {
	loaders := fromContext(ctx)
	return loaders.auditLogProjectID, loaders.auditLogLocation
}

func NewPostgresWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*PostgresInstance] {
	w := watcher.Watch(mgr, &PostgresInstance{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toPostgres(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "data.nais.io",
		Version:  "v1",
		Resource: "postgres",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
