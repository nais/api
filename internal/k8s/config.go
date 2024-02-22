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
