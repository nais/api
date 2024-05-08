package unleash

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
)

const (
	UnleashTeamsEnvVar = "TEAMS_ALLOWED_TEAMS"
	// @TODO this should be moved to config
	ManagementClusterNamespace = "devteam"
	ManagementClusterChangeme  = "dev"

	UnleashCustomImageRepo    = "europe-north1-docker.pkg.dev/nais-io/nais/images/"
	UnleashCustomImageName    = "unleash-v4"
	UnleashRequestCPU         = "100m"
	UnleashRequestMemory      = "128Mi"
	UnleashLimitMemory        = "256Mi"
	SqlProxyImage             = "gcr.io/cloudsql-docker/gce-proxy:1.19.1"
	SqlProxyRequestCPU        = "10m"
	SqlProxyRequestMemory     = "100Mi"
	SqlProxyLimitMemory       = "100Mi"
	DatabasePoolMax           = "3"
	DatabasePoolIdleTimeoutMs = "1000"
	LogLevel                  = "warn"
)

// @TODO decide how we want to specify which team can manage Unleash from Console
func hasAccessToUnleash(team string, unleash *unleash_nais_io_v1.Unleash) bool {
	for _, env := range unleash.Spec.ExtraEnvVars {
		if env.Name == UnleashTeamsEnvVar {
			teams := strings.Split(env.Value, ",")
			for _, t := range teams {
				if t == team {
					return true
				}
			}
		}
	}

	return false
}

// @TODO this should use management cluster and not tenant clusters
func (m *Manager) Unleash(ctx context.Context, team string) (*model.Unleash, error) {
	for _, informers := range m.clientMap {
		for _, informer := range informers.informers {
			objs, err := informer.Lister().ByNamespace(team).List(labels.Everything())
			if err != nil {
				return nil, err
			}

			for _, obj := range objs {
				unleashInstance := &unleash_nais_io_v1.Unleash{}
				if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, unleashInstance); err != nil {
					return nil, err
				}

				if hasAccessToUnleash(team, unleashInstance) {
					return model.ToUnleashInstance(unleashInstance), nil
				}
			}
		}
	}
	return nil, nil
}

// @TODO check if unleash already exists
func (m *Manager) CreateUnleash(ctx context.Context, team slug.Slug) (*model.Unleash, error) {
	client, found := m.clientMap[ManagementClusterChangeme]
	if !found {
		return nil, fmt.Errorf("no kubernetes client found for %s cluster", ManagementClusterChangeme)
	}

	unleashSpec := unleashSpec(team)
	unleashModel := model.ToUnleashInstance(unleashSpec)
	unleashObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unleashSpec)
	if err != nil {
		return nil, err
	}

	_, err = client.dynamicClient.Resource(unleash_nais_io_v1.GroupVersion.WithResource("unleashs")).Namespace(ManagementClusterNamespace).Create(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return unleashModel, nil
}

func unleashSpec(team slug.Slug) *unleash_nais_io_v1.Unleash {
	cloudSqlProto := corev1.ProtocolTCP
	cloudSqlPort := intstr.FromInt(3307)

	name := team.String()

	webIngressHost := fmt.Sprintf("%s-unleash-web.example.com", name)
	webIngressClass := "ingress-nginx"
	apiIngessHost := fmt.Sprintf("%s-unleash-api.example.com", name)
	apiIngressClass := "ingress-nginx"
	teamsApiUrl := "https://teams-api.example.com"
	teamsApiSecretName := "teams-api-secret"
	teamsApiSecretKey := "secret"
	googleProjectId := "example"
	sqlRegion := "europe-north1"
	sqlId := "example"
	sqlUserSericeAccount := "example"

	sqlCidr := "1.2.3.4/32"

	googleIapAudience := "fixme"

	return &unleash_nais_io_v1.Unleash{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Unleash",
			APIVersion: "unleash.nais.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ManagementClusterNamespace,
		},
		Spec: unleash_nais_io_v1.UnleashSpec{
			Size: 1,
			Database: unleash_nais_io_v1.UnleashDatabaseConfig{
				Host:                  "localhost",
				Port:                  "5432",
				SSL:                   "false",
				SecretName:            name,
				SecretUserKey:         "POSTGRES_USER",
				SecretPassKey:         "POSTGRES_PASSWORD",
				SecretDatabaseNameKey: "POSTGRES_DB",
			},
			WebIngress: unleash_nais_io_v1.UnleashIngressConfig{
				Enabled: true,
				Host:    webIngressHost,
				Path:    "/",
				Class:   webIngressClass,
			},
			ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{
				Enabled: true,
				Host:    apiIngessHost,
				// Allow access to /health endpoint, change to /api when https://github.com/nais/unleasherator/issues/100 is resolved
				Path:  "/",
				Class: apiIngressClass,
			},
			NetworkPolicy: unleash_nais_io_v1.UnleashNetworkPolicyConfig{
				Enabled:  true,
				AllowDNS: true,
				ExtraEgressRules: []networkingv1.NetworkPolicyEgressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{{
							Protocol: &cloudSqlProto,
							Port:     &cloudSqlPort,
						}},
						To: []networkingv1.NetworkPolicyPeer{{
							IPBlock: &networkingv1.IPBlock{
								CIDR: sqlCidr,
							},
						}},
					},
				},
			},
			ExtraEnvVars: []corev1.EnvVar{{
				Name:  "GOOGLE_IAP_AUDIENCE",
				Value: googleIapAudience,
			}, {
				Name:  "TEAMS_API_URL",
				Value: teamsApiUrl,
			}, {
				Name: "TEAMS_API_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: teamsApiSecretName,
						},
						Key: teamsApiSecretKey,
					},
				},
			}, {
				Name:  "TEAMS_ALLOWED_TEAMS",
				Value: name,
			}, {
				Name:  "LOG_LEVEL",
				Value: "info",
			}, {
				Name:  "DATABASE_POOL_MAX",
				Value: "3",
			}, {
				Name:  "DATABASE_POOL_IDLE_TIMEOUT_MS",
				Value: "100",
			}},
			ExtraContainers: []corev1.Container{{
				Name:  "sql-proxy",
				Image: SqlProxyImage,
				Args: []string{
					"--structured-logs",
					"--port=5432",
					fmt.Sprintf("%s:%s:%s", googleProjectId,
						sqlRegion,
						sqlId),
				},
				SecurityContext: &corev1.SecurityContext{
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{"ALL"},
					},
					Privileged:               boolRef(false),
					RunAsUser:                int64Ref(65532),
					RunAsNonRoot:             boolRef(true),
					AllowPrivilegeEscalation: boolRef(false),
				},
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(SqlProxyRequestCPU),
						corev1.ResourceMemory: resource.MustParse(SqlProxyRequestMemory),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse(SqlProxyLimitMemory),
					},
				},
			}},
			ExistingServiceAccountName: sqlUserSericeAccount,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse(UnleashRequestCPU),
					corev1.ResourceMemory: resource.MustParse(UnleashRequestMemory),
				},
				Limits: corev1.ResourceList{
					corev1.ResourceMemory: resource.MustParse(UnleashLimitMemory),
				},
			},
		},
	}
}
func boolRef(b bool) *bool {
	boolVar := b
	return &boolVar
}

func int64Ref(i int64) *int64 {
	intvar := i
	return &intvar
}
