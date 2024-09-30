package vulnerabilities

import (
	"fmt"

	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	Client
	prometheusClients clusterPrometheusClients
	cfg               *Config
}

type Config struct {
	DependencyTrack DependencyTrackConfig
	Prometheus      PrometheusConfig
}

type (
	clusterPrometheusClients map[string]VulnerabilityPrometheus
)

func NewManager(cfg *Config) *Manager {
	dependencytrackClient := NewDependencyTrackClient(
		cfg.DependencyTrack,
		log.WithField("client", "dependencytrack"),
	)

	prometheusClientMap, err := cfg.prometheusClients()
	if err != nil {
		log.WithError(err).Fatal("Failed to create prometheus clients")
	}

	manager := &Manager{
		Client:            dependencytrackClient,
		prometheusClients: prometheusClientMap,
		cfg:               cfg,
	}

	return manager
}

func (c *Config) prometheusClients() (clusterPrometheusClients, error) {
	clients := clusterPrometheusClients{}
	for _, cluster := range c.Prometheus.Clusters {
		if c.Prometheus.EnableFakes {
			clients[cluster] = NewFakePrometheusClient()
			continue
		}
		prometheusUrl := fmt.Sprintf("https://nais-prometheus.%s.%s.cloud.nais.io", cluster, c.Prometheus.Tenant)
		promClient, err := promapi.NewClient(promapi.Config{
			Address: prometheusUrl,
		})
		if err != nil {
			return nil, err
		}
		clients[cluster] = promv1.NewAPI(promClient)
	}

	return clients, nil
}
