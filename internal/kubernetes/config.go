package kubernetes

import (
	"fmt"
	"maps"
	"net/http"
	"strings"

	"github.com/nais/api/internal/slug"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"
)

type StaticCluster struct {
	Name  string
	Host  string
	Token string
}

// ClusterType is the kind of Kubernetes cluster Nais runs on. The cluster type determines (among other things) the
// format of the OIDC issuer URL used to validate projected ServiceAccount tokens.
type ClusterType string

const (
	ClusterTypeGKE    ClusterType = "gke"
	ClusterTypeOnprem ClusterType = "onprem"
)

// IssuerURLFormatGKE is the format string used to construct the OIDC issuer URL for a GKE cluster. It accepts
// (cluster, tenant). TODO: replace with the real format once we have it.
const IssuerURLFormatGKE = "https://container.googleapis.com/v1/projects/%[2]s/locations/europe-north1/clusters/%[1]s"

// IssuerURLFormatOnprem is the format string used to construct the OIDC issuer URL for an on-prem cluster. It
// accepts (cluster, tenant). TODO: replace with the real format once we have it.
const IssuerURLFormatOnprem = "https://kubernetes.%[1]s.%[2]s.cloud.nais.io"

// IssuerURL returns the OIDC issuer URL for the given cluster and tenant.
func IssuerURL(clusterType ClusterType, cluster, tenant string) string {
	switch clusterType {
	case ClusterTypeOnprem:
		return fmt.Sprintf(IssuerURLFormatOnprem, cluster, tenant)
	default:
		return fmt.Sprintf(IssuerURLFormatGKE, cluster, tenant)
	}
}

type ClusterConfigMap map[string]*rest.Config

func CreateClusterConfigMap(tenant string, clusters []string, staticClusters []StaticCluster) (ClusterConfigMap, error) {
	configs := ClusterConfigMap{}

	for _, cluster := range clusters {
		configs[cluster] = &rest.Config{
			Host: fmt.Sprintf("https://apiserver.%s.%s.cloud.nais.io", cluster, tenant),
			AuthProvider: &api.AuthProviderConfig{
				Name: GoogleAuthPlugin,
			},
			WrapTransport: func(rt http.RoundTripper) http.RoundTripper {
				return otelhttp.NewTransport(rt, otelhttp.WithServerName(cluster))
			},
		}
	}

	staticConfigs := getStaticClusterConfigs(staticClusters)
	maps.Copy(configs, staticConfigs)

	return configs, nil
}

func getStaticClusterConfigs(clusters []StaticCluster) ClusterConfigMap {
	configs := ClusterConfigMap{}
	for _, cluster := range clusters {
		configs[cluster.Name] = &rest.Config{
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
	return configs
}

func (c *StaticCluster) EnvDecode(value string) error {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, "|")
	if len(parts) != 3 {
		return fmt.Errorf(`invalid static cluster entry: %q. Must be on format "name|host|token"`, value)
	}

	name := strings.TrimSpace(parts[0])
	if name == "" {
		return fmt.Errorf("invalid static cluster entry: %q. Name must not be empty", value)
	}

	host := strings.TrimSpace(parts[1])
	if host == "" {
		return fmt.Errorf("invalid static cluster entry: %q. Host must not be empty", value)
	}

	token := strings.TrimSpace(parts[2])
	if token == "" {
		return fmt.Errorf("invalid static cluster entry: %q. Token must not be empty", value)
	}

	*c = StaticCluster{
		Name:  name,
		Host:  host,
		Token: token,
	}
	return nil
}

func (c ClusterConfigMap) TeamClient(environmentName string, teamSlug slug.Slug) (dynamic.Interface, error) {
	cfg, ok := c[environmentName]
	if !ok {
		return nil, fmt.Errorf("unknown environment: %q", environmentName)
	}

	impersonatedCfg := rest.CopyConfig(cfg)
	impersonatedCfg.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%[1]v:serviceuser-%[1]v", teamSlug),
	}

	return dynamic.NewForConfig(impersonatedCfg)
}
