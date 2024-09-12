package vulnerabilities

import (
	"context"
	"encoding/json"
	"github.com/nais/dependencytrack/pkg/client"
	"os"
)

func NewFakeDependencyTrackClient(c client.Client) client.Client {
	return &fakeDependencyTrackClient{c}
}

type fakeDependencyTrackClient struct {
	client.Client
}

func (f *fakeDependencyTrackClient) GetProjectsByTag(ctx context.Context, tag string) ([]*client.Project, error) {
	b, err := os.ReadFile("data/dependencytrack/projects.json")
	if err != nil {
		return nil, err
	}

	var projects []*client.Project
	err = json.Unmarshal(b, &projects)
	if err != nil {
		return nil, err
	}

	return projects, nil
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)
