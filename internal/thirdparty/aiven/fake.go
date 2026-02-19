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

//go:fix inline
func stringPtr(s string) *string {
	return new(s)
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
