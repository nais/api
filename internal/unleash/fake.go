package unleash

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/nais/api/internal/test"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

const (
	ManagementClusterNamespace = "bifrost-unleash"
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

type FakeBifrostClient struct {
	k8sClient dynamic.Interface
}

var _ BifrostClient = &FakeBifrostClient{}

func (f FakeBifrostClient) Post(ctx context.Context, path string, v any) (*http.Response, error) {
	unleashConfig := v.(bifrost.UnleashConfig)
	unleashInstance, err := f.CreateUnleash(ctx, unleashConfig)
	if err != nil {
		return nil, err
	}

	unleashJSON, err := json.Marshal(unleashInstance)
	if err != nil {
		return nil, err
	}

	return test.Response("200 OK", string(unleashJSON)), nil
}

func (f FakeBifrostClient) WithClient(_ *http.Client) {

}

func NewFakeBifrostClient(k8sClient dynamic.Interface) BifrostClient {
	return &FakeBifrostClient{k8sClient: k8sClient}
}

func (m *FakeBifrostClient) CreateUnleash(ctx context.Context, unleashConfig bifrost.UnleashConfig) (*unleash_nais_io_v1.Unleash, error) {
	client := m.k8sClient

	unleashSpec := unleashSpec(unleashConfig)
	unleashObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unleashSpec)
	if err != nil {
		return nil, err
	}

	_, err = client.Resource(unleash_nais_io_v1.GroupVersion.WithResource("unleashes")).Namespace("bifrost-unleash").Create(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return unleashSpec, nil
}

func unleashSpec(unleashConfig bifrost.UnleashConfig) *unleash_nais_io_v1.Unleash {
	name := unleashConfig.Name

	webIngressHost := fmt.Sprintf("%s-unleash-web.example.com", name)
	apiIngessHost := fmt.Sprintf("%s-unleash-api.example.com", name)

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
			WebIngress: unleash_nais_io_v1.UnleashIngressConfig{
				Host: webIngressHost,
			},
			ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{
				Host: apiIngessHost,
			},
			ExtraEnvVars: []corev1.EnvVar{{
				Name:  "TEAMS_ALLOWED_TEAMS",
				Value: unleashConfig.AllowedTeams,
			}},
		},
		Status: unleash_nais_io_v1.UnleashStatus{
			Reconciled: true,
			Connected:  true,
			Version:    "9.9.9",
		},
	}
}
