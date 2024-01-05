package k8s

import (
	"fmt"

	"github.com/nais/api/internal/config"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

type ClusterConfigMap map[string]rest.Config

func CreateClusterConfigMap(tenant string, cfg config.K8S) (ClusterConfigMap, error) {
	configs := ClusterConfigMap{}

	for _, cluster := range cfg.Clusters {
		configs[cluster] = rest.Config{
			Host: fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", cluster, tenant),
			AuthProvider: &api.AuthProviderConfig{
				Name: googleAuthPlugin,
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

func getStaticClusterConfigs(clusters []config.StaticCluster) (ClusterConfigMap, error) {
	configs := ClusterConfigMap{}
	for _, cluster := range clusters {
		configs[cluster.Name] = rest.Config{
			Host:        cluster.Host,
			BearerToken: cluster.Token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: true,
			},
		}
	}
	return configs, nil
}
