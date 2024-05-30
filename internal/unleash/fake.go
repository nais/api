package unleash

import (
	"context"
	"encoding/json"
	"fmt"
	"k8s.io/apimachinery/pkg/api/resource"
	"net/http"
	"strings"
	"time"

	"github.com/nais/api/internal/test"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

const (
	ManagementClusterNamespace = "bifrost-unleash"
)

type FakeBifrostClient struct {
	k8sClient dynamic.Interface
}

type FakePrometheusClient struct{}

var (
	_ BifrostClient = &FakeBifrostClient{}
	_ Prometheus    = &FakePrometheusClient{}
)

func NewFakePrometheusClient() Prometheus {
	return &FakePrometheusClient{}
}

func (f FakePrometheusClient) Query(_ context.Context, query string, _ time.Time, _ ...promv1.Option) (model.Value, promv1.Warnings, error) {
	val := model.Vector{
		&model.Sample{
			Metric: model.Metric{
				model.MetricNameLabel: "test_metric",
			},
			Timestamp: 1234567,
		},
	}
	switch {
	case strings.Contains(query, "cpu"):
		val[0].Value = model.SampleValue(0.06)
	case strings.Contains(query, "memory"):
		val[0].Value = model.SampleValue(10000000)
	case strings.Contains(query, "toggles"):
		val[0].Value = model.SampleValue(3)
	case strings.Contains(query, "client_apps"):
		val[0].Value = model.SampleValue(1)
	default:
		val[0].Value = model.SampleValue(0)
	}
	return val, nil, nil
}

func NewFakeBifrostClient(k8sClient dynamic.Interface) BifrostClient {
	return &FakeBifrostClient{k8sClient: k8sClient}
}

func (f FakeBifrostClient) Post(ctx context.Context, path string, v any) (*http.Response, error) {
	unleashConfig := v.(bifrost.UnleashConfig)

	var unleashInstance *unleash_nais_io_v1.Unleash
	var err error
	switch path {
	case "/unleash/new":
		unleashInstance, err = f.createOrUpdateUnleash(ctx, unleashConfig)
	case "/unleash/edit":
		unleashInstance, err = f.createOrUpdateUnleash(ctx, unleashConfig)
	default:
		return nil, fmt.Errorf("unknown path: %s", path)
	}

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

func (f FakeBifrostClient) createOrUpdateUnleash(ctx context.Context, config bifrost.UnleashConfig) (*unleash_nais_io_v1.Unleash, error) {
	client := f.k8sClient

	unleashSpec := unleashSpec(config)
	unleashObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unleashSpec)
	if err != nil {
		return nil, err
	}

	resource := client.Resource(unleash_nais_io_v1.GroupVersion.WithResource("unleashes")).Namespace("bifrost-unleash")

	if _, err = resource.Get(ctx, config.Name, metav1.GetOptions{}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			_, err = resource.Create(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.CreateOptions{})
			if err != nil {
				return nil, err
			}
			return nil, err
		}
	}

	_, err = resource.Update(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.UpdateOptions{})
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
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		},
		Status: unleash_nais_io_v1.UnleashStatus{
			Reconciled: true,
			Connected:  true,
			Version:    "9.9.9",
		},
	}
}
