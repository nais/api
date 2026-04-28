package application

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], ingressWatcher *watcher.Watcher[*netv1.Ingress], client IngressMetricsClient, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(appWatcher, ingressWatcher, client, log))
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
	client         IngressMetricsClient
	log            logrus.FieldLogger
}

func newLoaders(appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], ingressWatcher *watcher.Watcher[*netv1.Ingress], client IngressMetricsClient, log logrus.FieldLogger) *loaders {
	return &loaders{
		appWatcher:     appWatcher,
		ingressWatcher: ingressWatcher,
		client:         client,
		log:            log,
	}
}

func transformIngress(in any) (any, error) {
	ingress := in.(*unstructured.Unstructured)
	src := ingress.Object

	// metadata
	srcMeta, _ := src["metadata"].(map[string]any)
	newMeta := map[string]any{}
	if srcMeta != nil {
		for _, k := range []string{"name", "namespace", "uid", "resourceVersion"} {
			if v, ok := srcMeta[k]; ok {
				newMeta[k] = v
			}
		}
	}

	// metadata.labels - keep only "app" and "team"
	if labels, _, _ := unstructured.NestedStringMap(src, "metadata", "labels"); len(labels) > 0 {
		kept := map[string]any{}
		for _, k := range []string{"app", "team"} {
			if v, ok := labels[k]; ok {
				kept[k] = v
			}
		}
		if len(kept) > 0 {
			newMeta["labels"] = kept
		}
	}

	// spec.rules - keep only the Host field of each rule.
	rules, _, err := unstructured.NestedSlice(src, "spec", "rules")
	if err != nil {
		return nil, err
	}
	newRules := make([]any, 0, len(rules))
	for _, rule := range rules {
		r, ok := rule.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("rule is not a map[string]any")
		}
		newRules = append(newRules, map[string]any{
			"host": r["host"],
		})
	}

	newSpec := map[string]any{
		"rules": newRules,
	}
	if ingressClassName, found, _ := unstructured.NestedString(src, "spec", "ingressClassName"); found {
		newSpec["ingressClassName"] = ingressClassName
	}

	newObj := map[string]any{
		"apiVersion": src["apiVersion"],
		"kind":       src["kind"],
		"metadata":   newMeta,
		"spec":       newSpec,
	}
	ingress.Object = newObj
	return ingress, nil
}
