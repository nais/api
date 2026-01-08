package unleash

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/bifrost/pkg/bifrostclient"
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

func (f *FakeBifrostClient) CreateInstance(ctx context.Context, req bifrostclient.UnleashConfigRequest) (*bifrostclient.CreateInstanceResponse, error) {
	unleashInstance, err := f.createOrUpdateUnleash(ctx, req)
	if err != nil {
		return nil, err
	}

	bifrostUnleash := k8sToBifrostUnleash(unleashInstance)
	return &bifrostclient.CreateInstanceResponse{
		JSON201: bifrostUnleash,
	}, nil
}

func (f *FakeBifrostClient) UpdateInstance(ctx context.Context, name string, req bifrostclient.UnleashConfigRequest) (*bifrostclient.UpdateInstanceResponse, error) {
	unleashInstance, err := f.updateUnleash(ctx, name, req)
	if err != nil {
		return nil, err
	}

	bifrostUnleash := k8sToBifrostUnleash(unleashInstance)
	return &bifrostclient.UpdateInstanceResponse{
		JSON200: bifrostUnleash,
	}, nil
}

func (f *FakeBifrostClient) GetInstance(_ context.Context, name string) (*bifrostclient.GetInstanceResponse, error) {
	instance, err := f.watcher.Get("management", ManagementClusterNamespace, name)
	if err != nil {
		return nil, err
	}

	apiVersion := "unleash.nais.io/v1"
	kind := "Unleash"
	version := "9.9.9"
	connected := true

	return &bifrostclient.GetInstanceResponse{
		JSON200: &bifrostclient.Unleash{
			ApiVersion: &apiVersion,
			Kind:       &kind,
			Metadata: &struct {
				CreationTimestamp *time.Time `json:"creationTimestamp,omitempty"`
				Name              *string    `json:"name,omitempty"`
				Namespace         *string    `json:"namespace,omitempty"`
			}{
				Name: &instance.Name,
			},
			Status: &struct {
				Connected *bool   `json:"connected,omitempty"`
				Version   *string `json:"version,omitempty"`
			}{
				Version:   &version,
				Connected: &connected,
			},
		},
	}, nil
}

func (f *FakeBifrostClient) DeleteInstance(ctx context.Context, name string) (*bifrostclient.DeleteInstanceResponse, error) {
	if err := f.watcher.Delete(ctx, "management", ManagementClusterNamespace, name); err != nil {
		return nil, err
	}
	return &bifrostclient.DeleteInstanceResponse{}, nil
}

func (f *FakeBifrostClient) ListInstances(_ context.Context) (*bifrostclient.ListInstancesResponse, error) {
	instances := []bifrostclient.Unleash{}
	return &bifrostclient.ListInstancesResponse{
		JSON200: &instances,
	}, nil
}

func (f *FakeBifrostClient) ListChannels(_ context.Context) (*bifrostclient.ListChannelsResponse, error) {
	stableType := "sequential"
	rapidType := "canary"
	regularType := "sequential"

	stableTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	rapidTime := time.Date(2024, 3, 20, 14, 15, 0, 0, time.UTC)
	regularTime := time.Date(2024, 3, 10, 8, 0, 0, 0, time.UTC)

	channels := []bifrostclient.ReleaseChannelResponse{
		{
			Name:           "stable",
			Image:          "unleash:5.11.0",
			CurrentVersion: "5.11.0",
			Type:           &stableType,
			LastUpdated:    &stableTime,
			CreatedAt:      stableTime,
		},
		{
			Name:           "rapid",
			Image:          "unleash:5.12.0-beta.1",
			CurrentVersion: "5.12.0-beta.1",
			Type:           &rapidType,
			LastUpdated:    &rapidTime,
			CreatedAt:      rapidTime,
		},
		{
			Name:           "regular",
			Image:          "unleash:5.10.2",
			CurrentVersion: "5.10.2",
			Type:           &regularType,
			LastUpdated:    &regularTime,
			CreatedAt:      regularTime,
		},
	}

	return &bifrostclient.ListChannelsResponse{
		JSON200: &channels,
	}, nil
}

func (f *FakeBifrostClient) GetChannel(_ context.Context, name string) (*bifrostclient.GetChannelResponse, error) {
	channelType := "sequential"
	lastUpdated := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)

	return &bifrostclient.GetChannelResponse{
		JSON200: &bifrostclient.ReleaseChannelResponse{
			Name:           name,
			Image:          "unleash:5.11.0",
			CurrentVersion: "5.11.0",
			Type:           &channelType,
			LastUpdated:    &lastUpdated,
			CreatedAt:      lastUpdated,
		},
	}, nil
}

func (f *FakeBifrostClient) createOrUpdateUnleash(ctx context.Context, req bifrostclient.UnleashConfigRequest) (*unleash_nais_io_v1.Unleash, error) {
	name := ""
	if req.Name != nil {
		name = *req.Name
	}
	allowedTeams := ""
	if req.AllowedTeams != nil {
		allowedTeams = *req.AllowedTeams
	}
	releaseChannelName := ""
	if req.ReleaseChannelName != nil {
		releaseChannelName = *req.ReleaseChannelName
	}

	unleashSpec := unleashSpecFromConfig(name, allowedTeams, releaseChannelName)
	unleashObject, err := runtime.DefaultUnstructuredConverter.ToUnstructured(unleashSpec)
	if err != nil {
		return nil, err
	}

	defClient, err := f.watcher.SystemAuthenticatedClient(ctx, "management")
	if err != nil {
		return nil, err
	}
	client := defClient.Namespace(ManagementClusterNamespace)
	if _, err := f.watcher.Get("management", ManagementClusterNamespace, name); errors.Is(err, &watcher.ErrorNotFound{}) {
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

func (f *FakeBifrostClient) updateUnleash(ctx context.Context, name string, req bifrostclient.UnleashConfigRequest) (*unleash_nais_io_v1.Unleash, error) {
	// Get existing instance to preserve fields not being updated
	existing, err := f.watcher.Get("management", ManagementClusterNamespace, name)
	if err != nil {
		return nil, err
	}

	// Start with existing values
	allowedTeams := ""
	for _, team := range existing.AllowedTeamSlugs {
		if allowedTeams != "" {
			allowedTeams += ","
		}
		allowedTeams += team.String()
	}
	releaseChannelName := ""
	if existing.releaseChannelName != nil {
		releaseChannelName = *existing.releaseChannelName
	}

	// Override with request values if provided
	if req.AllowedTeams != nil {
		allowedTeams = *req.AllowedTeams
	}
	if req.ReleaseChannelName != nil {
		releaseChannelName = *req.ReleaseChannelName
	}

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
				Value: allowedTeams,
			}},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
				Name: releaseChannelName,
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

func unleashSpecFromConfig(name, allowedTeams, releaseChannelName string) *unleash_nais_io_v1.Unleash {
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
				Value: allowedTeams,
			}},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
			ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
				Name: releaseChannelName,
			},
		},
		Status: unleash_nais_io_v1.UnleashStatus{
			Reconciled: true,
			Connected:  true,
			Version:    "9.9.9",
		},
	}
}

// k8sToBifrostUnleash converts a K8s Unleash object to the bifrostclient.Unleash type
func k8sToBifrostUnleash(u *unleash_nais_io_v1.Unleash) *bifrostclient.Unleash {
	if u == nil {
		return nil
	}

	apiVersion := u.APIVersion
	kind := u.Kind
	name := u.Name
	namespace := u.Namespace
	creationTimestamp := u.CreationTimestamp.Time
	version := u.Status.Version
	connected := u.Status.Connected
	releaseChannel := u.Spec.ReleaseChannel.Name

	// Extract allowed teams from ExtraEnvVars
	var allowedTeams *[]string
	for _, env := range u.Spec.ExtraEnvVars {
		if env.Name == "TEAMS_ALLOWED_TEAMS" && env.Value != "" {
			teams := strings.Split(env.Value, ",")
			allowedTeams = &teams
			break
		}
	}

	return &bifrostclient.Unleash{
		ApiVersion: &apiVersion,
		Kind:       &kind,
		Metadata: &struct {
			CreationTimestamp *time.Time `json:"creationTimestamp,omitempty"`
			Name              *string    `json:"name,omitempty"`
			Namespace         *string    `json:"namespace,omitempty"`
		}{
			CreationTimestamp: &creationTimestamp,
			Name:              &name,
			Namespace:         &namespace,
		},
		Spec: &struct {
			CustomImage *string `json:"customImage,omitempty"`
			Federation  *struct {
				AllowedClusters *[]string `json:"allowedClusters,omitempty"`
				AllowedTeams    *[]string `json:"allowedTeams,omitempty"`
				Enabled         *bool     `json:"enabled,omitempty"`
			} `json:"federation,omitempty"`
			ReleaseChannel *struct {
				Name *string `json:"name,omitempty"`
			} `json:"releaseChannel,omitempty"`
		}{
			ReleaseChannel: &struct {
				Name *string `json:"name,omitempty"`
			}{
				Name: &releaseChannel,
			},
			Federation: &struct {
				AllowedClusters *[]string `json:"allowedClusters,omitempty"`
				AllowedTeams    *[]string `json:"allowedTeams,omitempty"`
				Enabled         *bool     `json:"enabled,omitempty"`
			}{
				AllowedTeams: allowedTeams,
			},
		},
		Status: &struct {
			Connected *bool   `json:"connected,omitempty"`
			Version   *string `json:"version,omitempty"`
		}{
			Version:   &version,
			Connected: &connected,
		},
	}
}
