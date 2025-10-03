package aiven

import (
	"context"
	"strings"
	"time"

	"github.com/aiven/go-client-codegen/handler/project"
	aiven "github.com/aiven/go-client-codegen/handler/service"
)

type FakeAivenClient struct{}

func NewFakeAivenClient() *FakeAivenClient {
	return &FakeAivenClient{}
}

func (f *FakeAivenClient) ServiceMaintenanceStart(_ context.Context, _ string, _ string) error {
	return nil
}

// ProjectAlertsList list active alerts for a project
func (f *FakeAivenClient) ProjectAlertsList(ctx context.Context, p string) ([]project.AlertOut, error) {
	if strings.HasSuffix(p, "dev") || strings.HasSuffix(p, "dev-gcp") {
		return []project.AlertOut{
			{
				ServiceName: stringPtr("opensearch-myteam-name"),
				ServiceType: stringPtr("opensearch"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: stringPtr("opensearch-devteam-name"),
				ServiceType: stringPtr("opensearch"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: stringPtr("opensearch-sortteam-critical"),
				ServiceType: stringPtr("opensearch"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: stringPtr("valkey-devteam-name"),
				ServiceType: stringPtr("valkey"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: stringPtr("valkey-myteam-name"),
				ServiceType: stringPtr("valkey"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: stringPtr("valkey-sortteam-critical"),
				ServiceType: stringPtr("valkey"),
				Severity:    "critical",
				Event:       "error message from aiven",
			},
			{
				ServiceName: stringPtr("someservicetype-devteam-name"),
				ServiceType: stringPtr("someservicetype"),
				Severity:    "critical",
				Event:       "someservicetype has issue in aiven",
			},
		}, nil
	}
	return []project.AlertOut{}, nil
}

func stringPtr(s string) *string {
	return &s
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
		Metadata: map[string]any{
			"opensearch_version": "2.17.2",
		},
	}, nil
}
