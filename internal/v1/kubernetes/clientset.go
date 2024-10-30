package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewClientSets(clusterConfig ClusterConfigMap) (map[string]kubernetes.Interface, error) {
	k8sClientSets := make(map[string]kubernetes.Interface)
	for cluster, cfg := range clusterConfig {
		if cfg == nil {
			var err error
			cfg, err = rest.InClusterConfig()
			if err != nil {
				return nil, fmt.Errorf("get in-cluster config for cluster %q: %w", cluster, err)
			}
		}
		clientSet, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("create k8s client set for cluster %q: %w", cluster, err)
		}
		k8sClientSets[cluster] = clientSet
	}

	return k8sClientSets, nil
}
