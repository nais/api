package watcher

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

type WatchOption func(*watcherSettings)

func WithConverter(fn func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool)) WatchOption {
	return func(m *watcherSettings) {
		m.converter = fn
	}
}

func WithGVR(gvr schema.GroupVersionResource) WatchOption {
	return func(m *watcherSettings) {
		m.gvr = &gvr
	}
}

type watcherSettings struct {
	converter func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool)
	gvr       *schema.GroupVersionResource
}

type Watcher[T Object] struct {
	watchers        []*clusterWatcher[T]
	datastore       *DataStore[T]
	log             logrus.FieldLogger
	resourceCounter metric.Int64UpDownCounter
	watchedType     string
}

func newWatcher[T Object](mgr *Manager, obj T, settings *watcherSettings, log logrus.FieldLogger) *Watcher[T] {
	w := &Watcher[T]{
		datastore:       NewDataStore[T](),
		log:             log,
		resourceCounter: mgr.resourceCounter,
	}
	for cluster, client := range mgr.managers {
		watcher, gvr := newClusterWatcher(client, cluster, w, obj, settings, log.WithField("cluster", cluster))
		if !watcher.isRegistered {
			continue
		}
		w.watchedType = gvr.String()

		w.watchers = append(w.watchers, watcher)
		mgr.addCacheSync(watcher.informer.Informer().HasSynced)
	}
	return w
}

func (w *Watcher[T]) Start(ctx context.Context) {
	for _, watcher := range w.watchers {
		go watcher.Start(ctx)
	}
}

func (w *Watcher[T]) add(cluster string, obj T) {
	w.resourceCounter.Add(context.TODO(), 1, metric.WithAttributes(attribute.String("type", w.watchedType), attribute.String("action", "add")))
	w.log.WithFields(logrus.Fields{
		"cluster":   cluster,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Debug("Adding object")
	w.datastore.Add(cluster, obj)
}

func (w *Watcher[T]) remove(cluster string, obj T) {
	w.resourceCounter.Add(context.TODO(), 1, metric.WithAttributes(attribute.String("type", w.watchedType), attribute.String("action", "remove")))
	w.log.WithFields(logrus.Fields{
		"cluster":   cluster,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Debug("Removing object")
	w.datastore.Remove(cluster, obj)
}

func (w *Watcher[T]) update(cluster string, obj T) {
	w.resourceCounter.Add(context.TODO(), 1, metric.WithAttributes(attribute.String("type", w.watchedType), attribute.String("action", "update")))
	w.log.WithFields(logrus.Fields{
		"cluster":   cluster,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Debug("Updating object")
	w.datastore.Update(cluster, obj)
}

func (w *Watcher[T]) All() []*EnvironmentWrapper[T] {
	return w.datastore.All()
}

func (w *Watcher[T]) Get(cluster, namespace, name string) (T, error) {
	return w.datastore.Get(cluster, namespace, name)
}

func (w *Watcher[T]) GetByCluster(cluster string, filter ...Filter) []*EnvironmentWrapper[T] {
	return w.datastore.GetByCluster(cluster, filter...)
}

func (w *Watcher[T]) GetByNamespace(namespace string, filter ...Filter) []*EnvironmentWrapper[T] {
	return w.datastore.GetByNamespace(namespace, filter...)
}

func (w *Watcher[T]) Delete(ctx context.Context, cluster, namespace string, name string) error {
	for _, watcher := range w.watchers {
		if watcher.cluster == cluster {
			return watcher.Delete(ctx, namespace, name)
		}
	}

	return &ErrorNotFound{
		Cluster:   cluster,
		Namespace: namespace,
		Name:      name,
	}
}

func (w *Watcher[T]) ImpersonatedClient(ctx context.Context, cluster string, opts ...ImpersonatedClientOption) (dynamic.NamespaceableResourceInterface, error) {
	for _, watcher := range w.watchers {
		if watcher.cluster == cluster {
			return watcher.ImpersonatedClient(ctx, opts...)
		}
	}

	return nil, fmt.Errorf("no watcher for cluster %s", cluster)
}

func (w *Watcher[T]) SystemAuthenticatedClient(ctx context.Context, cluster string) (dynamic.NamespaceableResourceInterface, error) {
	for _, watcher := range w.watchers {
		if watcher.cluster == cluster {
			return watcher.SystemAuthenticatedClient(ctx)
		}
	}

	return nil, fmt.Errorf("no watcher for cluster %s", cluster)
}

func Objects[T Object](list []*EnvironmentWrapper[T]) []T {
	ret := make([]T, len(list))
	for i, obj := range list {
		ret[i] = obj.Obj
	}
	return ret
}
