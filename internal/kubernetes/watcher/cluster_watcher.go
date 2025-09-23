package watcher

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/user"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type Object interface {
	runtime.Object
	GetName() string
	GetNamespace() string
	GetLabels() map[string]string
}

type clusterWatcher[T Object] struct {
	manager       *clusterManager
	isRegistered  bool
	informer      informers.GenericInformer
	cluster       string
	watcher       *Watcher[T]
	log           logrus.FieldLogger
	converterFunc func(o *unstructured.Unstructured, environmentName string) (obj any, ok bool)
	gvr           schema.GroupVersionResource
	quickDelete   bool
}

func newClusterWatcher[T Object](mgr *clusterManager, cluster string, watcher *Watcher[T], obj T, settings *watcherSettings, log logrus.FieldLogger) (*clusterWatcher[T], schema.GroupVersionResource) {
	inf, gvr, err := mgr.createInformer(obj, settings.gvr, settings.filterLabelSelector)
	if err != nil {
		mgr.log.WithError(err).Error("creating informer")
		return &clusterWatcher[T]{
			manager:      mgr,
			isRegistered: false,
		}, gvr
	}

	w := &clusterWatcher[T]{
		manager:       mgr,
		isRegistered:  true,
		informer:      inf,
		watcher:       watcher,
		cluster:       cluster,
		log:           log,
		converterFunc: settings.converter,
		gvr:           gvr,
		quickDelete:   settings.quckDelete,
	}

	if settings.transformer != nil {
		if err := inf.Informer().SetTransform(settings.transformer); err != nil {
			panic(err)
		}
	}

	if _, err := inf.Informer().AddEventHandler(w); err != nil {
		panic(err)
	}

	return w, gvr
}

func (w *clusterWatcher[T]) Start(ctx context.Context) {
	if !w.isRegistered {
		return
	}
	w.informer.Informer().Run(ctx.Done())
}

func (w *clusterWatcher[T]) convert(obj *unstructured.Unstructured) (T, bool) {
	if obj.GetNamespace() == "nais-system" {
		var def T
		return def, false
	}
	if w.converterFunc != nil {
		o, ok := w.converterFunc(obj, w.cluster)
		if !ok {
			var def T
			return def, false
		}
		return o.(T), true
	}

	var t T
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &t); err != nil {
		w.log.
			WithError(err).
			WithField("target", fmt.Sprintf("%T", obj)).
			Error("converting object")
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
	a, ok := obj.(cache.DeletedFinalStateUnknown)
	if ok {
		obj = a.Obj
	}

	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		w.log.WithField("type", fmt.Sprintf("%T", obj)).Warn("could not convert object to unstructured")
		return
	}
	t, ok := w.convert(u)
	if !ok {
		return
	}
	w.watcher.remove(w.cluster, t)
}

func (w *clusterWatcher[T]) Delete(ctx context.Context, namespace, name string) error {
	client, err := w.ImpersonatedClient(ctx)
	if err != nil {
		return fmt.Errorf("impersonating client: %w", err)
	}

	if w.quickDelete {
		fmt.Println("EXTRA DEBUG")
		obj, err := w.informer.Lister().ByNamespace(namespace).Get(name)
		if err == nil {
			return err
		}
		w.log.Warnf("Using quick delete for %T", obj)

		// Remove from informer cache to avoid waiting for resync to see the deletion
		err = w.informer.Informer().GetIndexer().Delete(obj)
		if err != nil {
			w.log.WithError(err).WithField("namespace", namespace).WithField("name", name).Warn("deleting object from informer cache")
		}
		err = w.informer.Informer().GetStore().Delete(obj)
		if err != nil {
			w.log.WithError(err).WithField("namespace", namespace).WithField("name", name).Warn("deleting object from informer store")
		}
	} else if _, ok := w.manager.client.(*fake.FakeDynamicClient); ok {
		// This is a hack to make sure that the object is removed from the datastore
		// when running with a fake client.
		// The events created by the fake client are using the wrong type when calling
		// watchers.
		obj, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			w.OnDelete(obj)
		}
	}
	return client.Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{})
}

func (w *clusterWatcher[T]) Client() dynamic.NamespaceableResourceInterface {
	return w.manager.client.Resource(w.gvr)
}

type ImpersonatedClientOption func(s *impersonatedSettings)

type impersonatedSettings struct {
	gvr *schema.GroupVersionResource
}

func WithImpersonatedClientGVR(gvr schema.GroupVersionResource) ImpersonatedClientOption {
	return func(s *impersonatedSettings) {
		s.gvr = &gvr
	}
}

func (w *clusterWatcher[T]) ImpersonatedClient(ctx context.Context, opts ...ImpersonatedClientOption) (dynamic.NamespaceableResourceInterface, error) {
	actor := authz.ActorFromContext(ctx)

	groups, err := user.ListGCPGroupsForUser(ctx, actor.User.GetID())
	if err != nil {
		return nil, fmt.Errorf("listing GCP groups for user: %w", err)
	}

	settings := &impersonatedSettings{}
	for _, opt := range opts {
		opt(settings)
	}

	gvr := w.gvr
	if settings.gvr != nil {
		gvr = *settings.gvr
	}

	if _, ok := w.manager.client.(*fake.FakeDynamicClient); ok {
		// Instead of configuring a custom client creator when using fake clients, we just
		// type check the client and return it if it's a fake client.
		w.log.WithField("groups", groups).Warn("impersonation is not supported in fake mode, but would impersonate with these groups")
		return w.manager.client.Resource(gvr), nil
	}

	cfg := rest.CopyConfig(w.manager.config)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: actor.User.Identity(),
		Groups:   groups,
	}

	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	return client.Resource(gvr), nil
}

func (w *clusterWatcher[T]) SystemAuthenticatedClient(ctx context.Context, opts ...ImpersonatedClientOption) (dynamic.NamespaceableResourceInterface, error) {
	settings := &impersonatedSettings{}
	for _, opt := range opts {
		opt(settings)
	}

	gvr := w.gvr
	if settings.gvr != nil {
		gvr = *settings.gvr
	}

	return w.manager.client.Resource(gvr), nil
}
