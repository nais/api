package watcher

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/pingcap/errors"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

type WatchOption func(*watcherSettings)

func WithConverter(fn func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool)) WatchOption {
	return func(m *watcherSettings) {
		m.converter = fn
	}
}

func WithTransformer(fn cache.TransformFunc) WatchOption {
	return func(m *watcherSettings) {
		m.transformer = fn
	}
}

func WithGVR(gvr schema.GroupVersionResource) WatchOption {
	return func(m *watcherSettings) {
		m.gvr = &gvr
	}
}

type watcherSettings struct {
	converter   func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool)
	transformer cache.TransformFunc
	gvr         *schema.GroupVersionResource
}

type Watcher[T Object] struct {
	watchers        []*clusterWatcher[T]
	log             logrus.FieldLogger
	resourceCounter metric.Int64UpDownCounter
	watchedType     string
}

func newWatcher[T Object](mgr *Manager, obj T, settings *watcherSettings, log logrus.FieldLogger) *Watcher[T] {
	w := &Watcher[T]{
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
}

func (w *Watcher[T]) remove(cluster string, obj T) {
	w.resourceCounter.Add(context.TODO(), 1, metric.WithAttributes(attribute.String("type", w.watchedType), attribute.String("action", "remove")))
	w.log.WithFields(logrus.Fields{
		"cluster":   cluster,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Debug("Removing object")
}

func (w *Watcher[T]) update(cluster string, obj T) {
	w.resourceCounter.Add(context.TODO(), 1, metric.WithAttributes(attribute.String("type", w.watchedType), attribute.String("action", "update")))
	w.log.WithFields(logrus.Fields{
		"cluster":   cluster,
		"name":      obj.GetName(),
		"namespace": obj.GetNamespace(),
	}).Debug("Updating object")
}

func (w *Watcher[T]) All() []*EnvironmentWrapper[T] {
	ret := make([]*EnvironmentWrapper[T], 0)
	for _, wat := range w.watchers {
		objs, err := wat.informer.Lister().List(labels.Everything())
		if err != nil {
			w.log.WithError(err).Error("listing objects")
			continue
		}

		for _, obj := range objs {
			if o, ok := wat.convert(obj.(*unstructured.Unstructured)); ok {
				ret = append(ret, &EnvironmentWrapper[T]{
					Obj:     o,
					Cluster: wat.cluster,
				})
			}
		}

	}

	return ret
}

func (w *Watcher[T]) Get(cluster, namespace, name string) (T, error) {
	var nilish T
	for _, wat := range w.watchers {
		if wat.cluster != cluster {
			continue
		}

		obj, err := wat.informer.Lister().ByNamespace(namespace).Get(name)
		if err != nil {
			if errors.IsNotFound(err) {
				return nilish, &ErrorNotFound{
					Cluster:   cluster,
					Namespace: namespace,
					Name:      name,
				}
			}
			return nilish, err
		}

		if o, ok := wat.convert(obj.(*unstructured.Unstructured)); ok {
			return o, nil
		}
	}
	// return w.datastore.Get(cluster, namespace, name)
	return nilish, &ErrorNotFound{
		Cluster:   cluster,
		Namespace: namespace,
		Name:      name,
	}
}

func (w *Watcher[T]) GetByCluster(cluster string, filter ...Filter) []*EnvironmentWrapper[T] {
	opts := &filterOptions{
		labels: labels.Everything(),
	}
	for _, f := range filter {
		f(opts)
	}

	// return w.datastore.GetByCluster(cluster, filter...)
	ret := make([]*EnvironmentWrapper[T], 0)
	for _, wat := range w.watchers {
		if wat.cluster != cluster {
			continue
		}

		objs, err := wat.informer.Lister().List(opts.labels)
		if err != nil {
			w.log.WithError(err).Error("listing objects")
			continue
		}

		for _, obj := range objs {
			if o, ok := wat.convert(obj.(*unstructured.Unstructured)); ok {
				ret = append(ret, &EnvironmentWrapper[T]{
					Obj:     o,
					Cluster: wat.cluster,
				})
			}
		}
	}

	slices.SortFunc(ret, func(i, j *EnvironmentWrapper[T]) int {
		return strings.Compare(i.GetName(), j.GetName())
	})

	return ret
}

func (w *Watcher[T]) GetByNamespace(namespace string, filter ...Filter) []*EnvironmentWrapper[T] {
	opts := &filterOptions{
		labels: labels.Everything(),
	}
	for _, f := range filter {
		f(opts)
	}

	// return w.datastore.GetByNamespace(namespace, filter...)
	ret := make([]*EnvironmentWrapper[T], 0)
	for _, wat := range w.watchers {
		if len(opts.clusters) > 0 && !slices.Contains(opts.clusters, wat.cluster) {
			continue
		}

		objs, err := wat.informer.Lister().ByNamespace(namespace).List(opts.labels)
		if err != nil {
			w.log.WithError(err).Error("listing objects")
			continue
		}

		for _, obj := range objs {
			if o, ok := wat.convert(obj.(*unstructured.Unstructured)); ok {
				ret = append(ret, &EnvironmentWrapper[T]{
					Obj:     o,
					Cluster: wat.cluster,
				})
			}
		}
	}

	slices.SortFunc(ret, func(i, j *EnvironmentWrapper[T]) int {
		return strings.Compare(i.GetName(), j.GetName())
	})

	return ret
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
