package kubernetes

import (
	"fmt"
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

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

type ClusterConfigMap map[string]rest.Config

func CreateClusterConfigMap(tenant string, cfg Config) (ClusterConfigMap, error) {
	configs := ClusterConfigMap{}

	for _, cluster := range cfg.Clusters {
		configs[cluster] = rest.Config{
			Host: fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", cluster, tenant),
			AuthProvider: &api.AuthProviderConfig{
				Name: GoogleAuthPlugin,
			},
			WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
				return otelhttp.NewTransport(rt, otelhttp.WithServerName(cluster))
			},
		}
	}

	staticConfigs, err := getStaticClusterConfigs(cfg.StaticClusters)
	if err != nil {
		return nil, err
	}

	for cluster, cfg := range staticConfigs {
		configs[cluster] = cfg
	}

	return configs, nil
}

func getStaticClusterConfigs(clusters []StaticCluster) (ClusterConfigMap, error) {
	configs := ClusterConfigMap{}
	for _, cluster := range clusters {
		configs[cluster.Name] = rest.Config{
			Host:        cluster.Host,
			BearerToken: cluster.Token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: true,
			},
			WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
				return otelhttp.NewTransport(rt, otelhttp.WithServerName(cluster.Name))
			},
		}
	}
	return configs, nil
}
