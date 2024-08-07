package watcher

import (
	"context"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

type WatchOption func(*watcherSettings)

type watcherSettings struct {
	converter func(o *unstructured.Unstructured) (obj any, ok bool)
	gvr       *schema.GroupVersionResource
}

func WithConverter(fn func(o *unstructured.Unstructured) (obj any, ok bool)) WatchOption {
	return func(m *watcherSettings) {
		m.converter = fn
	}
}

func WithGVR(gvr schema.GroupVersionResource) WatchOption {
	return func(m *watcherSettings) {
		m.gvr = &gvr
	}
}

type Watcher[T Object] struct {
	watchers  []*clusterWatcher[T]
	datastore *DataStore[T]
	log       logrus.FieldLogger
}

func newWatcher[T Object](mgr *Manager, obj T, settings *watcherSettings, log logrus.FieldLogger) *Watcher[T] {
	w := &Watcher[T]{
		datastore: NewDataStore[T](),
		log:       log,
	}
	for cluster, client := range mgr.managers {
		w.watchers = append(w.watchers, newClusterWatcher(client, cluster, w, obj, settings, log.WithField("cluster", cluster)))
	}
	return w
}

func (w *Watcher[T]) Start(ctx context.Context) {
	for _, watcher := range w.watchers {
		go watcher.Start(ctx)
	}
}

func (w *Watcher[T]) WaitForReady(ctx context.Context, timeout time.Duration) bool {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var syncs []cache.InformerSynced
	for _, watcher := range w.watchers {
		if !watcher.isRegistered {
			continue
		}
		syncs = append(syncs, watcher.informer.Informer().HasSynced)
	}
	return cache.WaitForCacheSync(ctx.Done(), syncs...)
}

func (w *Watcher[T]) add(cluster string, obj T) {
	if w == nil {
		panic("watcher is nil")
	}
	w.log.Debug("Adding object", "cluster", cluster, "name", obj.GetName(), "namespace", obj.GetNamespace())
	w.datastore.Add(cluster, obj)
}

func (w *Watcher[T]) remove(cluster string, obj T) {
	w.log.Debug("Removing object", "cluster", cluster, "name", obj.GetName(), "namespace", obj.GetNamespace())
	w.datastore.Remove(cluster, obj)
}

func (w *Watcher[T]) update(cluster string, obj T) {
	w.log.Debug("Updating object", "cluster", cluster, "name", obj.GetName(), "namespace", obj.GetNamespace())
	w.datastore.Update(cluster, obj)
}

func (w *Watcher[T]) Get(cluster, namespace, name string) (T, error) {
	return w.datastore.Get(cluster, namespace, name)
}

func (w *Watcher[T]) GetByCluster(cluster string) []T {
	return w.datastore.GetByCluster(cluster)
}

func (w *Watcher[T]) GetByNamespace(namespace string) []T {
	return w.datastore.GetByNamespace(namespace)
}
