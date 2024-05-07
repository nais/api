package unleash

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/labels"
)

func (m *Manager) Unleash(ctx context.Context, team string) error {
	for cluster, informers := range m.clientMap {
		for _, informer := range informers.informers {
			objs, err := informer.Lister().ByNamespace(team).List(labels.Everything())
			if err != nil {
				return err
			}
			fmt.Printf("Cluster: %s, unleash: %v\n", cluster, objs)
		}
	}
	return nil
}
