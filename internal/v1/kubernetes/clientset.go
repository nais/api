package kubernetes

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
)

func NewClientSets(clusterConfig ClusterConfigMap) (map[string]kubernetes.Interface, error) {
	// TODO: Add support for fake clients

	k8sClientSets := make(map[string]kubernetes.Interface)
	for cluster, cfg := range clusterConfig {
		clientSet, err := kubernetes.NewForConfig(&cfg)
		if err != nil {
			return nil, fmt.Errorf("create k8s client set for cluster %q: %w", cluster, err)
		}
		k8sClientSets[cluster] = clientSet
	}

	return k8sClientSets, nil
}
