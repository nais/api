package config

import (
	"github.com/nais/api/internal/fixtures"
	"github.com/nais/api/internal/gcp"
)

type GitHub struct {
	// Organization The GitHub organization slug for the tenant.
	Organization string `envconfig:"API_BACKEND_GITHUB_ORG"`

	// AuthEndpoint Endpoint URL to the GitHub auth component.
	AuthEndpoint string `envconfig:"API_BACKEND_GITHUB_AUTH_ENDPOINT"`
}

type GCP struct {
	// Clusters A JSON-encoded value describing the GCP clusters to use. Refer to the README for the format.
	Clusters gcp.Clusters `envconfig:"API_BACKEND_GCP_CLUSTERS"`

	// CnrmRole The name of the custom CNRM role that is used when creating role bindings for the GCP projects of each
	// team. The value must also contain the organization ID.
	//
	// Example: `organizations/<org_id>/roles/CustomCNRMRole`, where `<org_id>` is a numeric ID.
	CnrmRole string `envconfig:"API_BACKEND_GCP_CNRM_ROLE"`

	// BillingAccount The ID of the billing account that each team project will be assigned to.
	//
	// Example: `billingAccounts/123456789ABC`
	BillingAccount string `envconfig:"API_BACKEND_GCP_BILLING_ACCOUNT"`

	// WorkloadIdentityPoolName The name of the workload identity pool used in the management project.
	//
	// Example: projects/{project_number}/locations/global/workloadIdentityPools/{workload_identity_pool_id}
	WorkloadIdentityPoolName string `envconfig:"API_BACKEND_GCP_WORKLOAD_IDENTITY_POOL_NAME"`
}

type NaisNamespace struct {
	// AzureEnabled When set to true api will send the Azure group ID of the team, if it has been created by
	// the Azure AD group reconciler, to naisd when creating a namespace for the NAIS team.
	AzureEnabled bool `envconfig:"API_BACKEND_NAIS_NAMESPACE_AZURE_ENABLED"`
}

type NaisDeploy struct {
	// Endpoint URL to the NAIS deploy key provisioning endpoint
	Endpoint string `envconfig:"API_BACKEND_NAIS_DEPLOY_ENDPOINT" default:"http://localhost:8080/api/v1/provision"`

	// ProvisionKey The API key used when provisioning deploy keys on behalf of NAIS teams.
	ProvisionKey string `envconfig:"API_BACKEND_NAIS_DEPLOY_PROVISION_KEY"`

	// DeployKeyEndpoint URL to the NAIS deploy key endpoint
	// DeployKeyEndpoint string `envconfig:"API_BACKEND_NAIS_DEPLOY_DEPLOY_KEY_ENDPOINT" default:"http://localhost:8080/internal/api/v1/apikey"`
}

type IAP struct {
	// IAP audience for validating IAP tokens
	Audience string `envconfig:"API_BACKEND_IAP_AUDIENCE"`

	// Insecure bypasses IAP authentication, just using the email header
	Insecure bool `envconfig:"API_BACKEND_IAP_INSECURE"`
}

type TeamsConfig struct {
	DependencyTrack DependencyTrack
	GitHub          GitHub
	GCP             GCP
	NaisDeploy      NaisDeploy
	NaisNamespace   NaisNamespace
	OAuth           OAuth
	IAP             IAP

	// Environments A list of environment names used for instance in GCP
	Environments []string

	// DatabaseURL The URL for the database.
	DatabaseURL string `envconfig:"API_BACKEND_DATABASE_URL" default:"postgres://api:api@localhost:3002/api?sslmode=disable"`

	// FrontendURL URL to the teams-frontend instance.
	FrontendURL string `envconfig:"API_BACKEND_FRONTEND_URL" default:"http://localhost:3001"`

	// Names of reconcilers to enable on first run of api
	//
	// Example: google:gcp:project,nais:namespace
	// Valid: [google:gcp:project|google:workspace-admin|nais:namespace|nais:deploy]
	FirstRunEnableReconcilers []fixtures.EnableableReconciler `envconfig:"API_BACKEND_FIRST_RUN_ENABLE_RECONCILERS"`

	// ListenAddress The host:port combination used by the http server.
	ListenAddress string `envconfig:"API_BACKEND_LISTEN_ADDRESS" default:"127.0.0.1:3000"`

	// LogFormat Customize the log format. Can be "text" or "json".
	LogFormat string `envconfig:"API_BACKEND_LOG_FORMAT" default:"text"`

	// LogLevel The log level used in api.
	LogLevel string `envconfig:"API_BACKEND_LOG_LEVEL" default:"DEBUG"`

	// GoogleManagementProjectID The ID of the NAIS management project in the tenant organization in GCP.
	GoogleManagementProjectID string `envconfig:"API_BACKEND_GOOGLE_MANAGEMENT_PROJECT_ID"`

	// OnpremClusters a list of onprem clusters (NAV only)
	// Example: "dev-fss,prod-fss,ci-fss"
	OnpremClusters []string `envconfig:"API_BACKEND_ONPREM_CLUSTERS"`

	// StaticServiceAccounts A JSON-encoded value describing a set of service accounts to be created when the
	// application starts. Refer to the README for the format.
	StaticServiceAccounts fixtures.ServiceAccounts `envconfig:"API_BACKEND_STATIC_SERVICE_ACCOUNTS"`

	// TenantDomain The domain for the tenant.
	TenantDomain string `envconfig:"API_BACKEND_TENANT_DOMAIN" default:"example.com"`

	// TenantName The name of the tenant.
	TenantName string `envconfig:"API_BACKEND_TENANT_NAME" default:"example"`
}
