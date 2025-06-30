package fake

import (
	"context"
	"time"

	aiven "github.com/aiven/go-client-codegen/handler/service"
)

type FakeAivenClient struct{}

func NewFakeAivenClient() *FakeAivenClient {
	return &FakeAivenClient{}
}

func (f *FakeAivenClient) ServiceMaintenanceStart(_ context.Context, _ string, _ string) error {
	return nil
}

// ServiceGet returns hardcoded example dataset
func (f *FakeAivenClient) ServiceGet(_ context.Context, _ string, _ string, _ ...[2]string) (*aiven.ServiceGetOut, error) {
	description := "This is a description (Nais API call it title)"
	link := "https://nais.io"
	impact := "This is the impact (Nais API call it description)"
	startAt := time.Date(1987, 7, 9, 0, 0, 0, 0, time.UTC)
	deadline := startAt.Add(24 * time.Hour).Format(time.RFC3339)
	startAfter := startAt.Add(1 * time.Hour).Format(time.RFC3339)

	return &aiven.ServiceGetOut{
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
	}, nil
}
