package environmentmapper

import "sync"

// EnvironmentMapping is a mapping between cluster names used in Kubernetes (keys) and user facing environment
// names (values). Only used by the nav.no tenant.
type EnvironmentMapping map[string]string

var (
	mapping EnvironmentMapping
	lock    sync.RWMutex
)

// SetMapping initializes the environment mapping.
func SetMapping(environmentMapping EnvironmentMapping) {
	lock.Lock()
	defer lock.Unlock()
	mapping = environmentMapping
}

// EnvironmentName takes a cluster name used in Kubernetes and returns a user facing environment name.
func EnvironmentName(clusterName string) string {
	lock.RLock()
	defer lock.RUnlock()

	if replacement, ok := mapping[clusterName]; ok {
		return replacement
	}

	return clusterName
}

// ClusterName takes a user facing environment name and returns the cluster name used in Kubernetes.
func ClusterName(environmentName string) string {
	lock.RLock()
	defer lock.RUnlock()

	for k, v := range mapping {
		if v == environmentName {
			return k
		}
	}

	return environmentName
}
