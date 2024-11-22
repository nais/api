package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], ingressWatcher *watcher.Watcher[*netv1.Ingress]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(appWatcher, ingressWatcher))
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*nais_io_v1alpha1.Application] {
	w := watcher.Watch(mgr, &nais_io_v1alpha1.Application{})
	w.Start(ctx)
	return w
}

func NewIngressWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*netv1.Ingress] {
	w := watcher.Watch(mgr, &netv1.Ingress{}, watcher.WithTransformer(transformIngress))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	appWatcher     *watcher.Watcher[*nais_io_v1alpha1.Application]
	ingressWatcher *watcher.Watcher[*netv1.Ingress]
}

func newLoaders(appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], ingressWatcher *watcher.Watcher[*netv1.Ingress]) *loaders {
	return &loaders{
		appWatcher:     appWatcher,
		ingressWatcher: ingressWatcher,
	}
}

func transformIngress(in any) (any, error) {
	fieldsToRemove := [][]string{
		{"spec"},
		{"status"},
		{"metadata", "creationTimestamp"},
		{"metadata", "generateName"},
		{"metadata", "ownerReferences"},
		{"metadata", "annotations"},
		{"metadata", "managedFields"},
		{"status", "initContainerStatuses"},
	}

	labelsToKeep := []string{
		"app",
		"team",
	}

	ingress := in.(*unstructured.Unstructured)
	// Getting data to keep
	rules, _, err := unstructured.NestedSlice(ingress.Object, "spec", "rules")
	if err != nil {
		return nil, err
	}

	ingressClassName, _, _ := unstructured.NestedString(ingress.Object, "spec", "ingressClassName")

	// We only need to keep Host
	newRules := []any{}
	for _, rule := range rules {
		r, ok := rule.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("rule is not a map[string]any")
		}
		newRule := map[string]any{
			"host": r["host"],
		}
		newRules = append(newRules, newRule)
	}

	// Removing data
	for _, field := range fieldsToRemove {
		unstructured.RemoveNestedField(ingress.Object, field...)
	}

	labels := ingress.GetLabels()
	for k := range labels {
		if !slices.Contains(labelsToKeep, k) {
			delete(labels, k)
		}
	}
	ingress.SetLabels(labels)

	// Adding data back
	unstructured.SetNestedSlice(ingress.Object, newRules, "spec", "rules")
	unstructured.SetNestedField(ingress.Object, ingressClassName, "spec", "ingressClassName")

	return ingress, nil
}
