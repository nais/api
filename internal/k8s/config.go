package k8s

// Config is the configuration related to Kubernetes
type Config struct {
	Clusters       []string
	StaticClusters []StaticCluster
}

type StaticCluster struct {
	Name  string
	Host  string
	Token string
}

func (c *Config) IsStaticCluster(cluster string) bool {
	for _, sc := range c.StaticClusters {
		if sc.Name == cluster {
			return true
		}
	}
	return false
}
