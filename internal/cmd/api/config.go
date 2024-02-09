package api

import (
	"context"

	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/k8s"
	"github.com/sethvargo/go-envconfig"
)

type StaticCluster struct {
	Name  string
	Host  string
	Token string
}

type k8sConfig struct {
	Clusters       []string        `env:"KUBERNETES_CLUSTERS"`
	StaticClusters []StaticCluster `env:"KUBERNETES_CLUSTERS_STATIC"`
}

func (k *k8sConfig) AllClusterNames() []string {
	clusters := append([]string{}, k.Clusters...)
	for _, c := range k.StaticClusters {
		clusters = append(clusters, c.Name)
	}
	return clusters
}

func (k *k8sConfig) GraphClusterList() graph.ClusterList {
	clusters := make(graph.ClusterList)
	for _, cluster := range k.Clusters {
		clusters[cluster] = graph.ClusterInfo{
			GCP: true,
		}
	}
	for _, staticCluster := range k.StaticClusters {
		clusters[staticCluster.Name] = graph.ClusterInfo{}
	}

	return clusters
}

func (k *k8sConfig) PkgConfig() k8s.Config {
	return k8s.Config{
		Clusters: k.Clusters,
		StaticClusters: func() []k8s.StaticCluster {
			var clusters []k8s.StaticCluster
			for _, c := range k.StaticClusters {
				clusters = append(clusters, k8s.StaticCluster{
					Name:  c.Name,
					Host:  c.Host,
					Token: c.Token,
				})
			}
			return clusters
		}(),
	}
}

type userSyncConfig struct {
	// Enabled When set to true api will keep the user database in sync with the connected Google
	// organization. The Google organization will be treated as the master.
	Enabled bool `env:"USERSYNC_ENABLED"`

	// RunsToPersist Number of runs to store for the userSync GraphQL query.
	RunsToPersist int `env:"USERSYNC_RUNS_TO_PERSIST,default=5"`

	// AdminGroupPrefix The prefix of the admin group email address.
	// TODO: change default value to nais-admins (or something similar) and rename existing groups
	AdminGroupPrefix string `env:"USERSYNC_ADMIN_GROUP_PREFIX,default=nais-admins"`
}

// costConfig is the configuration for the cost service
type costConfig struct {
	ImportEnabled     bool   `env:"COST_DATA_IMPORT_ENABLED"`
	BigQueryProjectID string `env:"BIGQUERY_PROJECTID,default=*detect-project-id*"`
}

// dependencyTrackConfig is the configuration for the dependency track service
type dependencyTrackConfig struct {
	Endpoint string `env:"DEPENDENCYTRACK_ENDPOINT,default=http://dependencytrack-backend:8080"`
	Frontend string `env:"DEPENDENCYTRACK_FRONTEND"`
	// TODO: change default value to something other than console
	Username string `env:"DEPENDENCYTRACK_USERNAME,default=console"`
	Password string `env:"DEPENDENCYTRACK_PASSWORD"`
}

// hookdConfig is the configuration for the hookd service
type hookdConfig struct {
	Endpoint string `env:"HOOKD_ENDPOINT,default=http://hookd"`
	PSK      string `env:"HOOKD_PSK"`
}

type oAuthConfig struct {
	// ClientID The ID of the OAuth 2.0 client to use for the OAuth login flow.
	ClientID string `env:"OAUTH_CLIENT_ID"`

	// ClientSecret The client secret to use for the OAuth login flow.
	ClientSecret string `env:"OAUTH_CLIENT_SECRET"`

	// RedirectURL The URL that Google will redirect back to after performing authentication.
	RedirectURL string `env:"OAUTH_REDIRECT_URL"`

	// FrontendURL The URL of the frontend application.
	// TODO: This should be removed as we are always on the same domain
	FrontendURL string `env:"OAUTH_FRONTEND_URL"`
}

type Config struct {
	// Tenant is the active tenant
	Tenant string `env:"TENANT,default=dev-nais"`

	// TenantDomain The domain for the tenant.
	TenantDomain string `env:"TENANT_DOMAIN,default=example.com"`

	// GoogleManagementProjectID The ID of the NAIS management project in the tenant organization in GCP.
	GoogleManagementProjectID string `env:"GOOGLE_MANAGEMENT_PROJECT_ID"`

	// DatabaseConnectionString is the database DSN
	DatabaseConnectionString string `env:"DATABASE_URL,default=postgres://api:api@127.0.0.1:3002/api?sslmode=disable"`

	LogFormat string `env:"LOG_FORMAT,default=json"`
	LogLevel  string `env:"LOG_LEVEL,default=info"`

	// StaticServiceAccounts A JSON-encoded value describing a set of service accounts to be created when the
	// application starts. Refer to the README for the format.
	StaticServiceAccounts fixtures.ServiceAccounts `env:"STATIC_SERVICE_ACCOUNTS"`

	// ResourceUtilization is the configuration for the resource utilization service
	ResourceUtilizationImportEnabled bool `env:"RESOURCE_UTILIZATION_IMPORT_ENABLED"`

	// WithFakeKubernetes When set to true, the api will use a fake kubernetes client.
	WithFakeClients bool `env:"WITH_FAKE_CLIENTS"`

	// ListenAddress is host:port combination used by the http server
	ListenAddress string `env:"LISTEN_ADDRESS,default=127.0.0.1:3000"`

	// GRPCListenAddress is host:port combination used by the GRPC server
	GRPCListenAddress string `env:"GRPC_LISTEN_ADDRESS,default=127.0.0.1:3001"`

	K8s             k8sConfig
	UserSync        userSyncConfig
	Cost            costConfig
	DependencyTrack dependencyTrackConfig
	Hookd           hookdConfig
	OAuth           oAuthConfig
}

// NewConfig creates a new configuration instance from environment variables
func NewConfig(ctx context.Context, lookuper envconfig.Lookuper) (*Config, error) {
	cfg := &Config{}
	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	})
	if err != nil {
		return nil, err
	}

	return cfg, nil
}
