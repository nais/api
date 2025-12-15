package unleash

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/test"
	bifrost "github.com/nais/bifrost/pkg/unleash"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	ManagementClusterNamespace = "bifrost-unleash"
)

type FakeBifrostClient struct {
	watcher *watcher.Watcher[*UnleashInstance]
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
		fmt.Println("QUERY", query)
		val[0].Value = model.SampleValue(0)
	}
	return val, nil, nil
}

func NewFakeBifrostClient(wtchr *watcher.Watcher[*UnleashInstance]) BifrostClient {
	return &FakeBifrostClient{watcher: wtchr}
}

func (f FakeBifrostClient) Post(ctx context.Context, path string, v any) (*http.Response, error) {
	var unleashInstance *unleash_nais_io_v1.Unleash
	var err error

	switch path {
	case "/unleash/new", "/v1/unleash":
		var config BifrostV1CreateRequest
		if v0Config, ok := v.(bifrost.UnleashConfig); ok {
			config = BifrostV1CreateRequest{
				Name:             v0Config.Name,
				AllowedTeams:     v0Config.AllowedTeams,
				EnableFederation: v0Config.EnableFederation,
				AllowedClusters:  v0Config.AllowedClusters,
			}
		} else if v1Config, ok := v.(BifrostV1CreateRequest); ok {
			config = v1Config
		} else {
			return nil, fmt.Errorf("unknown config type for path: %s", path)
		}
		unleashInstance, err = f.createOrUpdateUnleash(ctx, config)
	default:
		if strings.HasPrefix(path, "/unleash/") && strings.HasSuffix(path, "/edit") {
			var config BifrostV1UpdateRequest
			if v0Config, ok := v.(bifrost.UnleashConfig); ok {
				config = BifrostV1UpdateRequest{
					AllowedTeams: v0Config.AllowedTeams,
				}
			} else if v1Config, ok := v.(BifrostV1UpdateRequest); ok {
				config = v1Config
			} else {
				return nil, fmt.Errorf("unknown config type for path: %s", path)
			}
			parts := strings.Split(path, "/")
			if len(parts) >= 3 {
				name := parts[2]
				unleashInstance, err = f.updateUnleash(ctx, name, config)
			} else {
				return nil, fmt.Errorf("invalid edit path: %s", path)
			}
		} else {
			return nil, fmt.Errorf("unknown path: %s", path)
		}
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

func (f FakeBifrostClient) Put(ctx context.Context, path string, v any) (*http.Response, error) {
	if !strings.HasPrefix(path, "/v1/unleash/") {
		return nil, fmt.Errorf("unknown PUT path: %s", path)
	}

	name := strings.TrimPrefix(path, "/v1/unleash/")

	var config BifrostV1UpdateRequest
	if v1Config, ok := v.(BifrostV1UpdateRequest); ok {
		config = v1Config
	} else {
		return nil, fmt.Errorf("unknown config type for PUT: %T", v)
	}

	unleashInstance, err := f.updateUnleash(ctx, name, config)
	if err != nil {
		return nil, err
	}

	unleashJSON, err := json.Marshal(unleashInstance)
	if err != nil {
		return nil, err
	}

	return test.Response("200 OK", string(unleashJSON)), nil
}

func (f FakeBifrostClient) Get(_ context.Context, path string) (*http.Response, error) {
	if path == "/v1/releasechannels" {
		channels := []BifrostV1ReleaseChannelResponse{
			{
				Name:           "stable",
				Version:        "5.11.0",
				Type:           "sequential",
				Description:    "Stable release channel with tested versions",
				CurrentVersion: "5.11.0",
				LastUpdated:    "2024-03-15T10:30:00Z",
			},
			{
				Name:           "rapid",
				Version:        "5.12.0-beta.1",
				Type:           "canary",
				Description:    "Rapid release channel with latest features",
				CurrentVersion: "5.12.0-beta.1",
				LastUpdated:    "2024-03-20T14:15:00Z",
			},
			{
				Name:           "regular",
				Version:        "5.10.2",
				Type:           "sequential",
				Description:    "Regular release channel with conservative updates",
				CurrentVersion: "5.10.2",
				LastUpdated:    "2024-03-10T08:00:00Z",
			},
		}

		channelsJSON, err := json.Marshal(channels)
		if err != nil {
			return nil, err
		}

		return test.Response("200 OK", string(channelsJSON)), nil
	}

	return nil, fmt.Errorf("unknown GET path: %s", path)
}

func (f FakeBifrostClient) WithClient(_ *http.Client) {
}

func (f FakeBifrostClient) createOrUpdateUnleash(ctx context.Context, config BifrostV1CreateRequest) (*unleash_nais_io_v1.Unleash, error) {
	unleashSpec := unleashSpecFromV1Config(config)
	unleashObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unleashSpec)
	if err != nil {
		return nil, err
	}

	defClient, err := f.watcher.SystemAuthenticatedClient(ctx, "management")
	if err != nil {
		return nil, err
	}
	client := defClient.Namespace(ManagementClusterNamespace)
	if _, err := f.watcher.Get("management", ManagementClusterNamespace, config.Name); errors.Is(err, &watcher.ErrorNotFound{}) {
		_, err := client.Create(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		return unleashSpec, nil
	}
	_, err = client.Update(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return unleashSpec, nil
}

func (f FakeBifrostClient) updateUnleash(ctx context.Context, name string, config BifrostV1UpdateRequest) (*unleash_nais_io_v1.Unleash, error) {
	unleashSpec := &unleash_nais_io_v1.Unleash{
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
				Host: fmt.Sprintf("%s-unleash-web.example.com", name),
			},
			ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{
				Host: fmt.Sprintf("%s-unleash-api.example.com", name),
			},
			ExtraEnvVars: []corev1.EnvVar{{
				Name:  "TEAMS_ALLOWED_TEAMS",
				Value: config.AllowedTeams,
			}},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
				Name: config.ReleaseChannelName,
			},
		},
		Status: unleash_nais_io_v1.UnleashStatus{
			Reconciled: true,
			Connected:  true,
			Version:    "9.9.9",
		},
	}

	unleashObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unleashSpec)
	if err != nil {
		return nil, err
	}

	defClient, err := f.watcher.SystemAuthenticatedClient(ctx, "management")
	if err != nil {
		return nil, err
	}
	client := defClient.Namespace(ManagementClusterNamespace)

	_, err = client.Update(ctx, &unstructured.Unstructured{Object: unleashObject}, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}

	return unleashSpec, nil
}

func unleashSpecFromV1Config(config BifrostV1CreateRequest) *unleash_nais_io_v1.Unleash {
	name := config.Name

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
				Value: config.AllowedTeams,
			}},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
				Name: config.ReleaseChannelName,
			},
		},
		Status: unleash_nais_io_v1.UnleashStatus{
			Reconciled: true,
			Connected:  true,
			Version:    "9.9.9",
		},
	}
}
