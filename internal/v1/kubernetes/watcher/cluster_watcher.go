package watcher

import (
	"context"
	"fmt"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
)

type Object interface {
	runtime.Object
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
}

type clusterWatcher[T Object] struct {
	client        dynamic.Interface
	isRegistered  bool
	informer      informers.GenericInformer
	cluster       string
	watcher       *Watcher[T]
	log           logrus.FieldLogger
	converterFunc func(o *unstructured.Unstructured) (obj any, ok bool)
}

func newClusterWatcher[T Object](mgr *clusterManager, cluster string, watcher *Watcher[T], obj T, settings *watcherSettings, log logrus.FieldLogger) *clusterWatcher[T] {
	inf, err := mgr.createInformer(obj, settings.gvr)
	if err != nil {
		mgr.log.Error("creating informer", "error", err)
		return &clusterWatcher[T]{
			client:       mgr.client,
			isRegistered: false,
		}
	}

	w := &clusterWatcher[T]{
		client:        mgr.client,
		isRegistered:  true,
		informer:      inf,
		watcher:       watcher,
		cluster:       cluster,
		log:           log,
		converterFunc: settings.converter,
	}

	inf.Informer().AddEventHandler(w)

	return w
}

func (w *clusterWatcher[T]) Start(ctx context.Context) {
	if !w.isRegistered {
		return
	}
	w.informer.Informer().Run(ctx.Done())
}

func (w *clusterWatcher[T]) convert(obj *unstructured.Unstructured) (T, bool) {
	if w.converterFunc != nil {
		o, ok := w.converterFunc(obj)
		if !ok {
			var def T
			return def, false
		}
		return o.(T), true
	}

	var t T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &t); err != nil {
		w.log.Error("converting object", "error", err, "target", fmt.Sprintf("%T", obj))
		return t, false
	}
	return t, true
}

func (w *clusterWatcher[T]) OnAdd(obj any, isInInitialList bool) {
	t, ok := w.convert(obj.(*unstructured.Unstructured))
	if !ok {
		return
	}
	w.watcher.add(w.cluster, t)
}

func (w *clusterWatcher[T]) OnUpdate(oldObj, newObj any) {
	t, ok := w.convert(newObj.(*unstructured.Unstructured))
	if !ok {
		return
	}
	w.watcher.update(w.cluster, t)
}

func (w *clusterWatcher[T]) OnDelete(obj any) {
	t, ok := w.convert(obj.(*unstructured.Unstructured))
	if !ok {
		return
	}
	w.watcher.remove(w.cluster, t)
}
