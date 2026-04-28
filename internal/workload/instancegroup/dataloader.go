package instancegroup

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, rsWatcher *watcher.Watcher[*appsv1.ReplicaSet], podWatcher *watcher.Watcher[*corev1.Pod], appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application], k8sClients map[string]dynamic.Interface, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		rsWatcher:  rsWatcher,
		podWatcher: podWatcher,
		appWatcher: appWatcher,
		k8sClients: k8sClients,
		log:        log,
	})
}

func NewWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*appsv1.ReplicaSet] {
	w := watcher.Watch(mgr, &appsv1.ReplicaSet{}, watcher.WithTransformer(transformReplicaSet), watcher.WithInformerFilter("app"))
	w.Start(ctx)
	return w
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	rsWatcher  *watcher.Watcher[*appsv1.ReplicaSet]
	podWatcher *watcher.Watcher[*corev1.Pod]
	appWatcher *watcher.Watcher[*nais_io_v1alpha1.Application]
	k8sClients map[string]dynamic.Interface
	log        logrus.FieldLogger
}

// k8sClient returns a system-authenticated dynamic client for the given environment.
func (l *loaders) k8sClient(environmentName string) (dynamic.Interface, error) {
	clusterName := environmentmapper.ClusterName(environmentName)
	client, ok := l.k8sClients[clusterName]
	if !ok {
		return nil, fmt.Errorf("unknown environment: %s", environmentName)
	}
	return client, nil
}

// transformReplicaSet strips unnecessary fields from ReplicaSets but keeps
// the data we need: labels, annotations (revision), replicas, pod template
// (containers with env/envFrom/volumeMounts/image, volumes).
func transformReplicaSet(in any) (any, error) {
	rs := in.(*unstructured.Unstructured)
	src := rs.Object

	// metadata
	srcMeta, _ := src["metadata"].(map[string]any)
	newMeta := map[string]any{}
	if srcMeta != nil {
		for _, k := range []string{"name", "namespace", "uid", "resourceVersion", "creationTimestamp"} {
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

	// metadata.annotations - keep only deployment.kubernetes.io/revision
	if annotations, _, _ := unstructured.NestedStringMap(src, "metadata", "annotations"); len(annotations) > 0 {
		if v, ok := annotations["deployment.kubernetes.io/revision"]; ok {
			newMeta["annotations"] = map[string]any{
				"deployment.kubernetes.io/revision": v,
			}
		}
	}

	// metadata.ownerReferences - kept verbatim (links to Deployment/Application).
	// NestedSlice already deep-copies, so we can store the result directly.
	if ownerRefs, _, _ := unstructured.NestedSlice(src, "metadata", "ownerReferences"); len(ownerRefs) > 0 {
		newMeta["ownerReferences"] = ownerRefs
	}

	// spec
	newSpec := map[string]any{}
	if replicas, found, _ := unstructured.NestedInt64(src, "spec", "replicas"); found {
		newSpec["replicas"] = replicas
	}

	// spec.template.spec.containers - keep name, image, env, envFrom, volumeMounts
	containers, _, _ := unstructured.NestedSlice(src, "spec", "template", "spec", "containers")
	newContainers := make([]any, 0, len(containers))
	for _, container := range containers {
		c, ok := container.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("container is not a map[string]any")
		}
		newC := map[string]any{}
		for _, k := range []string{"name", "image", "env", "envFrom", "volumeMounts"} {
			if v, ok := c[k]; ok {
				newC[k] = v
			}
		}
		newContainers = append(newContainers, newC)
	}

	templateSpec := map[string]any{
		"containers": newContainers,
	}
	if volumes, _, _ := unstructured.NestedSlice(src, "spec", "template", "spec", "volumes"); len(volumes) > 0 {
		templateSpec["volumes"] = volumes
	}

	templateMeta := map[string]any{}
	if templateLabels, _, _ := unstructured.NestedStringMap(src, "spec", "template", "metadata", "labels"); len(templateLabels) > 0 {
		labelsAny := make(map[string]any, len(templateLabels))
		for k, v := range templateLabels {
			labelsAny[k] = v
		}
		templateMeta["labels"] = labelsAny
	}

	template := map[string]any{
		"spec": templateSpec,
	}
	if len(templateMeta) > 0 {
		template["metadata"] = templateMeta
	}
	newSpec["template"] = template

	// status - just the replica counters
	newStatus := map[string]any{}
	if v, found, _ := unstructured.NestedInt64(src, "status", "replicas"); found {
		newStatus["replicas"] = v
	}
	if v, found, _ := unstructured.NestedInt64(src, "status", "readyReplicas"); found {
		newStatus["readyReplicas"] = v
	}

	newObj := map[string]any{
		"apiVersion": src["apiVersion"],
		"kind":       src["kind"],
		"metadata":   newMeta,
		"spec":       newSpec,
		"status":     newStatus,
	}
	rs.Object = newObj
	return rs, nil
}
