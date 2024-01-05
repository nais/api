package config

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

// Cost is the configuration for the cost service
type Cost struct {
	ImportEnabled     bool   `env:"COST_DATA_IMPORT_ENABLED,default=false"`
	BigQueryProjectID string `env:"BIGQUERY_PROJECTID,default=*detect-project-id*"`
}

// Hookd is the configuration for the hookd service
type Hookd struct {
	Endpoint string `env:"HOOKD_ENDPOINT,default=http://hookd"`
	PSK      string `env:"HOOKD_PSK,default=secret-frontend-psk"`
}

// K8S is the configuration related to Kubernetes
type K8S struct {
	Clusters        []string        `env:"KUBERNETES_CLUSTERS"`
	StaticClusters  []StaticCluster `env:"KUBERNETES_CLUSTERS_STATIC"`
	AllClusterNames []string
}

// DependencyTrack is the configuration for the dependency track service
type DependencyTrack struct {
	Endpoint string `env:"DEPENDENCYTRACK_ENDPOINT,default=http://dependencytrack-backend:8080"`
	Frontend string `env:"DEPENDENCYTRACK_FRONTEND"`
	Username string `env:"DEPENDENCYTRACK_USERNAME,default=console"`
	Password string `env:"DEPENDENCYTRACK_PASSWORD"`
}

// Logger is the configuration for the logger
type Logger struct {
	Format string `env:"LOG_FORMAT,default=json"`
	Level  string `env:"LOG_LEVEL,default=info"`
}

// ResourceUtilization is the configuration for the resource utilization service
type ResourceUtilization struct {
	ImportEnabled bool `env:"RESOURCE_UTILIZATION_IMPORT_ENABLED,default=false"`
}

// Teams is the configuration for the teams backend service
type Teams struct {
	Endpoint string `env:"TEAMS_ENDPOINT,default=http://teams-backend/query"`
	Token    string `env:"TEAMS_TOKEN,default=secret-admin-api-key"`
}

// Config is the configuration for the api application
type Config struct {
	Cost                Cost
	Hookd               Hookd
	K8S                 K8S
	Logger              Logger
	DependencyTrack     DependencyTrack
	ResourceUtilization ResourceUtilization
	Teams               Teams

	// IapAudience is the audience for the IAP JWT token. Will not be used when RUN_AS_USER is set
	IapAudience string `env:"IAP_AUDIENCE"`

	// DatabaseConnectionString is the database DSN
	DatabaseConnectionString string `env:"CONSOLE_DATABASE_URL,default=postgres://postgres:postgres@127.0.0.1:5432/console?sslmode=disable"`

	// ListenAddress is host:port combination used by the http server
	ListenAddress string `env:"LISTEN_ADDRESS,default=:8080"`

	// RunAsUser is the static user to run as. Used for development purposes. Will override IAP_AUDIENCE when set
	RunAsUser string `env:"RUN_AS_USER"`

	// Tenant is the active tenant
	Tenant string `env:"TENANT,default=dev-nais"`
}

// New creates a new configuration instance from environment variables
func New(ctx context.Context, lookuper envconfig.Lookuper) (*Config, error) {
	cfg := &Config{}
	err := envconfig.ProcessWith(ctx, cfg, lookuper)
	if err != nil {
		return nil, err
	}
	if cfg.RunAsUser == "" && cfg.IapAudience == "" {
		return nil, fmt.Errorf("either RUN_AS_USER or IAP_AUDIENCE must be set")
	}

	clusterNames := cfg.K8S.Clusters
	for _, staticCluster := range cfg.K8S.StaticClusters {
		clusterNames = append(clusterNames, staticCluster.Name)
	}
	cfg.K8S.AllClusterNames = clusterNames

	return cfg, nil
}
