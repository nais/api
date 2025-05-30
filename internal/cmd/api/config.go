package api

import (
	"context"
	"reflect"

	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/workload/logging"
	"github.com/sethvargo/go-envconfig"
	"github.com/sirupsen/logrus"
)

type k8sConfig struct {
	Clusters       []string                   `env:"KUBERNETES_CLUSTERS"`
	StaticClusters []kubernetes.StaticCluster `env:"KUBERNETES_CLUSTERS_STATIC"`
}

func (k *k8sConfig) AllClusterNames() []string {
	clusters := append([]string{}, k.Clusters...)
	for _, c := range k.StaticClusters {
		clusters = append(clusters, c.Name)
	}
	return clusters
}

type ClusterInfo struct {
	GCP bool
}

type ClusterList map[string]ClusterInfo

func (k *k8sConfig) ClusterList() ClusterList {
	clusters := make(ClusterList)
	for _, cluster := range k.Clusters {
		clusters[cluster] = ClusterInfo{
			GCP: true,
		}
	}
	for _, staticCluster := range k.StaticClusters {
		clusters[staticCluster.Name] = ClusterInfo{}
	}

	return clusters
}

type usersyncConfig struct {
	// Enabled When set to true api will keep the user database in sync with the connected Google
	// organization. The Google organization will be treated as the master.
	Enabled bool `env:"USERSYNC_ENABLED"`

	// AdminGroupPrefix The prefix of the admin group email address.
	AdminGroupPrefix string `env:"USERSYNC_ADMIN_GROUP_PREFIX,default=console-admins"`

	// Service account to impersonate during user sync
	ServiceAccount string `env:"USERSYNC_SERVICE_ACCOUNT"`

	// SubjectEmail The email address to impersonate during user sync. This is an email address of a user
	// with the necessary permissions to read the Google organization.
	SubjectEmail string `env:"USERSYNC_SUBJECT_EMAIL"`
}

// costConfig is the configuration for the cost service
type costConfig struct {
	ImportEnabled     bool   `env:"COST_DATA_IMPORT_ENABLED"`
	BigQueryProjectID string `env:"BIGQUERY_PROJECTID,default=*detect-project-id*"`
}

// vulnerabilitiesConfig is the configuration for the vulnerability manager using the v13s api
type vulnerabilitiesConfig struct {
	Endpoint       string `env:"VULNERABILITIES_ENDPOINT,default=fake"`
	ServiceAccount string `env:"VULNERABILITIES_SERVICE_ACCOUNT,default=service-account"`
}

// hookdConfig is the configuration for the hookd service
type hookdConfig struct {
	Endpoint string `env:"HOOKD_ENDPOINT,default=http://hookd"`
	PSK      string `env:"HOOKD_PSK"`
}

type oAuthConfig struct {
	// Issuer The issuer of the OAuth 2.0 client to use for the OAuth login flow.
	Issuer string `env:"OAUTH_ISSUER,default=https://accounts.google.com"`

	// ClientID The ID of the OAuth 2.0 client to use for the OAuth login flow.
	ClientID string `env:"OAUTH_CLIENT_ID"`

	// ClientSecret The client secret to use for the OAuth login flow.
	ClientSecret string `env:"OAUTH_CLIENT_SECRET"`

	// RedirectURL The URL that Google will redirect back to after performing authentication.
	RedirectURL string `env:"OAUTH_REDIRECT_URL"`

	// AdditionalScopes is a list of additional scopes to request in the OAuth login flow.
	AdditionalScopes []string `env:"OAUTH_ADDITIONAL_SCOPES"`
}

type unleashConfig struct {
	// BifrostApiEndpoint is the endpoint for the Bifrost API
	BifrostApiUrl string `env:"UNLEASH_BIFROST_API_URL,default=*fake*"`
}

type loggingConfig struct {
	LokiDefault       bool `env:"LOGGING_LOKI_CLUSTER_DEFAULT"`
	SecureLogsDefault bool `env:"LOGGING_SECURE_LOGS_CLUSTER_DEFAULT"`
}

type zitadelConfig struct {
	// IDPID is the ID of the IDPID to use for the Zitadel API
	IDPID string `env:"ZITADEL_IDP_ID"`

	// Key is the secret key to use for the Zitadel API
	Key string `env:"ZITADEL_KEY"`

	// Domain is the domain to use for the Zitadel API
	Domain string `env:"ZITADEL_DOMAIN"`

	// OrganizationID is the ID of the organization to use for the Zitadel API
	OrganizationID string `env:"ZITADEL_ORGANIZATION_ID"`
}

func (l loggingConfig) DefaultLogDestinations() []logging.SupportedLogDestination {
	var destinations []logging.SupportedLogDestination
	if l.LokiDefault {
		destinations = append(destinations, logging.Loki)
	}
	if l.SecureLogsDefault {
		destinations = append(destinations, logging.SecureLogs)
	}
	return destinations
}

type Config struct {
	// Aiven token is the token for the aiven token
	AivenToken string `env:"AIVEN_TOKEN"`

	// Tenant is the active tenant
	Tenant string `env:"TENANT,default=dev-nais"`

	// TenantDomain The domain for the tenant.
	TenantDomain string `env:"TENANT_DOMAIN,default=example.com"`

	// GoogleManagementProjectID The ID of the Nais management project in the tenant organization in GCP.
	GoogleManagementProjectID string `env:"GOOGLE_MANAGEMENT_PROJECT_ID"`

	// DatabaseConnectionString is the database DSN
	DatabaseConnectionString string `env:"DATABASE_URL,default=postgres://api:api@127.0.0.1:3002/api?sslmode=disable"`

	LogFormat string `env:"LOG_FORMAT,default=json"`
	LogLevel  string `env:"LOG_LEVEL,default=info"`

	// StaticServiceAccounts A JSON-encoded value describing a set of service accounts to be created when the
	// application starts. Refer to the README for the format.
	StaticServiceAccounts StaticServiceAccounts `env:"STATIC_SERVICE_ACCOUNTS"`

	WithSlowQueryLogger bool `env:"WITH_SLOW_QUERY_LOGGER"`

	// ListenAddress is host:port combination used by the http server
	ListenAddress         string `env:"LISTEN_ADDRESS,default=127.0.0.1:3000"`
	InternalListenAddress string `env:"INTERNAL_LISTEN_ADDRESS,default=127.0.0.1:3005"`

	// GRPCListenAddress is host:port combination used by the GRPC server
	GRPCListenAddress string `env:"GRPC_LISTEN_ADDRESS,default=127.0.0.1:3001"`

	LeaseName      string `env:"LEASE_NAME,default=nais-api-lease"`
	LeaseNamespace string `env:"LEASE_NAMESPACE,default=nais-system"`

	// ReplaceEnvironmentNames is a map of cluster names to replace in the UI. Keys are cluster names used in
	// Kubernetes, for instance "prod", and the values are user-facing environment names, for instance "prod-gcp". This
	// configuration value is only used by the nav.no tenant.
	ReplaceEnvironmentNames map[string]string `env:"REPLACE_ENVIRONMENT_NAMES, noinit"`

	K8s                k8sConfig
	Usersync           usersyncConfig
	Cost               costConfig
	VulnerabilitiesApi vulnerabilitiesConfig
	Hookd              hookdConfig
	OAuth              oAuthConfig
	Unleash            unleashConfig
	Logging            loggingConfig
	Zitadel            zitadelConfig
	Fakes              Fakes
	JWT                JWTConfig
}

type Fakes struct {
	WithInsecureUserHeader bool `env:"WITH_INSECURE_USER_HEADER"`
	WithFakeAivenClient    bool `env:"WITH_FAKE_AIVEN_CLIENT"`
	WithFakeKubernetes     bool `env:"WITH_FAKE_KUBERNETES"`
	WithFakeHookd          bool `env:"WITH_FAKE_HOOKD"`
	WithFakeCloudSQL       bool `env:"WITH_FAKE_CLOUD_SQL"`
	WithFakePrometheus     bool `env:"WITH_FAKE_PROMETHEUS"`
	WithFakeCostClient     bool `env:"WITH_FAKE_COST_CLIENT"`
	WithFakePriceClient    bool `env:"WITH_FAKE_PRICE_CLIENT"`
}

func (f Fakes) Inform(log logrus.FieldLogger) {
	v := reflect.ValueOf(f)
	for i := range v.NumField() {
		field := v.Type().Field(i)
		if v.Field(i).Bool() {
			log.Warnf("%s is true", field.Name)
		}
	}
}

type JWTConfig struct {
	Issuer         string `env:"JWT_ISSUER,default=https://auth.nais.io"`
	Audience       string `env:"JWT_AUDIENCE,default=320114319427740585"`
	SkipMiddleware bool   `env:"JWT_SKIP_MIDDLEWARE,default=false"`
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
