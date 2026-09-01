package aiven

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/aiven/go-client-codegen/handler/project"
	aiven "github.com/aiven/go-client-codegen/handler/service"
)

type FakeAivenClient struct {
	lock     sync.RWMutex
	versions map[string]string
}

const (
	localDevOpenSearchVersion = "2.19.3"
	localDevValkeyVersion     = "8.1.9"
)

// localDevVersions gives the instances in data/k8s a version, so running against fakes
// answers the version field the way a real Aiven would. Keying by service name rather
// than answering for every service is deliberate: the integration tests share this
// constructor and several of them require a service Aiven reports no version for.
var localDevVersions = map[string]string{
	"opensearch-devteam-opensearch-1": localDevOpenSearchVersion,
	"opensearch-devteam-opensearch-2": localDevOpenSearchVersion,
	"opensearch-devteam-non-managed":  localDevOpenSearchVersion,
	"valkey-devteam-contests":         localDevValkeyVersion,
	"valkey-devteam-contests-managed": localDevValkeyVersion,
}

func NewFakeAivenClient() *FakeAivenClient {
	versions := make(map[string]string, len(localDevVersions))
	for serviceName, version := range localDevVersions {
		versions[serviceName] = version
	}
	return &FakeAivenClient{versions: versions}
}

// SetServiceVersion makes ServiceGet report version for serviceName. Tests declare the
// versions they need, so no single hardcoded value has to serve every test at once. A
// service nobody sets a version for reports none, which is how Aiven behaves before it
// has finished provisioning.
func (f *FakeAivenClient) SetServiceVersion(serviceName, version string) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.versions[serviceName] = version
}

func (f *FakeAivenClient) ServiceMaintenanceStart(_ context.Context, _ string, _ string) error {
	return nil
}

// ProjectAlertsList list active alerts for a project
func (f *FakeAivenClient) ProjectAlertsList(ctx context.Context, p string) ([]project.AlertOut, error) {
	if strings.HasSuffix(p, "dev") || strings.HasSuffix(p, "dev-gcp") {
		return []project.AlertOut{
			{
				ServiceName: new("opensearch-myteam-name"),
				ServiceType: new("opensearch"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: new("opensearch-devteam-name"),
				ServiceType: new("opensearch"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: new("opensearch-sortteam-critical"),
				ServiceType: new("opensearch"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: new("valkey-devteam-name"),
				ServiceType: new("valkey"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: new("valkey-myteam-name"),
				ServiceType: new("valkey"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: new("valkey-sortteam-critical"),
				ServiceType: new("valkey"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: new("someservicetype-devteam-name"),
				ServiceType: new("someservicetype"),
				Severity:    "critical",
				Event:       "someservicetype has issue in aiven",
			},
		}, nil
	}
	return []project.AlertOut{}, nil
}

// ServiceGet returns hardcoded example dataset
func (f *FakeAivenClient) ServiceGet(_ context.Context, _ string, serviceName string, _ ...[2]string) (*aiven.ServiceGetOut, error) {
	description := "This is a description (Nais API call it title)"
	link := "https://nais.io"
	impact := "This is the impact (Nais API call it description)"
	startAt := time.Date(1987, 7, 9, 0, 0, 0, 0, time.UTC)
	deadline := startAt.Add(24 * time.Hour).Format(time.RFC3339)
	startAfter := startAt.Add(1 * time.Hour).Format(time.RFC3339)

	state := aiven.ServiceStateTypeRunning
	if strings.HasSuffix(serviceName, "poweroff") {
		state = aiven.ServiceStateTypePoweroff
	} else if strings.HasSuffix(serviceName, "rebalancing") {
		state = aiven.ServiceStateTypeRebalancing
	}

	metadata := map[string]any{}
	f.lock.RLock()
	version, versionSet := f.versions[serviceName]
	f.lock.RUnlock()
	if versionSet {
		// Aiven keys the version by service type, which is also the service-name prefix.
		serviceType, _, _ := strings.Cut(serviceName, "-")
		metadata[serviceType+"_version"] = version
	}

	return &aiven.ServiceGetOut{
		State: state,
		Maintenance: &aiven.MaintenanceOut{
			Updates: []aiven.UpdateOut{
				{
					Description:       &description,
					DocumentationLink: &link,
					Impact:            &impact,
				},
				{
					Deadline:    &deadline,
					Description: &description,
					Impact:      &impact,
					StartAfter:  &startAfter,
					StartAt:     &startAt,
				},
			},
			Dow:  "sunday",
			Time: "12:34:56",
		},
		Metadata: metadata,
	}, nil
}
