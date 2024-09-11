package vulnerabilities

import (
	"context"
	"github.com/nais/dependencytrack/pkg/client"
)

func NewFakeDependencyTrackClient(c client.Client) client.Client {
	return &fakeDependencyTrackClient{c}
}

type fakeDependencyTrackClient struct {
	client.Client
}

func (f *fakeDependencyTrackClient) GetProjectsByTag(ctx context.Context, tag string) ([]*client.Project, error) {

	projects := make([]*client.Project, 0)
	projects = append(projects, &client.Project{
		Name:    "",
		Tags:    nil,
		Uuid:    "",
		Version: "",
		Metrics: &client.ProjectMetric{
			Critical:             0,
			High:                 0,
			Medium:               0,
			Low:                  0,
			Unassigned:           0,
			Vulnerabilities:      0,
			VulnerableComponents: 0,
			Components:           0,
			Suppressed:           0,
			FindingsTotal:        0,
			FindingsAudited:      0,
			FindingsUnaudited:    0,
			InheritedRiskScore:   0,
			FirstOccurrence:      0,
			LastOccurrence:       0,
		},
	})
	return projects, nil
	//return f.Client.GetProjectsByTag(ctx, tag)
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)
