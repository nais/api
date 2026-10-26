package postgres

import (
	"context"
	"fmt"
	"maps"

	"github.com/nais/api/internal/kubernetes/watcher"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

// CNPGCluster contains the PostgreSQL parameters explicitly configured on a
// CloudNativePG Cluster. These are desired settings, not runtime pg_settings;
// absent parameters are not filled in with PostgreSQL or operator defaults.
type CNPGCluster struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Parameters map[string]string
}

func (c *CNPGCluster) DeepCopyObject() runtime.Object {
	if c == nil {
		return (*CNPGCluster)(nil)
	}
	copy := *c
	copy.ObjectMeta = *c.ObjectMeta.DeepCopy()
	copy.Parameters = maps.Clone(c.Parameters)
	return &copy
}

// NewCNPGClusterWatcher watches the underlying CloudNativePG clusters without
// depending on the CNPG controller's types.
func NewCNPGClusterWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*CNPGCluster] {
	w := watcher.Watch(mgr, &CNPGCluster{},
		watcher.WithGVR(cnpgClusterGVR),
		watcher.WithConverter(func(o *unstructured.Unstructured, _ string) (any, bool) {
			cluster, err := toCNPGCluster(o)
			return cluster, err == nil
		}),
	)
	w.Start(ctx)
	return w
}

func toCNPGCluster(o *unstructured.Unstructured) (*CNPGCluster, error) {
	parameters, _, err := unstructured.NestedStringMap(o.Object, "spec", "postgresql", "parameters")
	if err != nil {
		return nil, fmt.Errorf("reading PostgreSQL parameters from CNPG Cluster %q: %w", o.GetName(), err)
	}
	return &CNPGCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: o.GetAPIVersion(), Kind: o.GetKind()},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.GetName(), Namespace: o.GetNamespace(), Labels: o.GetLabels(),
		},
		Parameters: parameters,
	}, nil
}
