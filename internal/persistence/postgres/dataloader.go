package postgres

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(
	ctx context.Context,
	postgresBranchWatcher *watcher.Watcher[*PostgresBranch],
	auditLogProjectID string,
	auditLogLocation string,
	tenantName string,
	clients map[string]dynamic.Interface,
) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(postgresBranchWatcher, auditLogProjectID, auditLogLocation, tenantName, clients))
}

type loaders struct {
	postgresBranchWatcher *watcher.Watcher[*PostgresBranch]
	auditLogProjectID     string
	auditLogLocation      string
	tenantName            string
	clients               map[string]dynamic.Interface
}

func newLoaders(
	postgresBranchWatcher *watcher.Watcher[*PostgresBranch],
	auditLogProjectID string,
	auditLogLocation string,
	tenantName string,
	clients map[string]dynamic.Interface,
) *loaders {
	return &loaders{
		postgresBranchWatcher: postgresBranchWatcher,
		auditLogProjectID:     auditLogProjectID,
		auditLogLocation:      auditLogLocation,
		tenantName:            tenantName,
		clients:               clients,
	}
}

// GetAuditLogConfig returns the audit log configuration from context
func GetAuditLogConfig(ctx context.Context) (projectID, location string) {
	loaders := fromContext(ctx)
	return loaders.auditLogProjectID, loaders.auditLogLocation
}

func NewPostgresBranchWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*PostgresBranch] {
	w := watcher.Watch(mgr, &PostgresBranch{}, watcher.WithConverter(func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool) {
		ret, err := toPostgresBranch(o, environmentName)
		if err != nil {
			return nil, false
		}
		return ret, true
	}), watcher.WithGVR(schema.GroupVersionResource{
		Group:    "nais.io",
		Version:  "v1",
		Resource: "postgresbranches",
	}))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
