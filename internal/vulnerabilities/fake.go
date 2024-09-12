package vulnerabilities

import (
	"context"
	"encoding/json"
	"github.com/nais/dependencytrack/pkg/client"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
)

func NewFakeDependencyTrackClient(c client.Client) client.Client {

	b, err := os.ReadFile("data/dependencytrack/devteam.json")
	if err != nil {
		log.Fatalf("failed to read fake data: %s", err)
	}

	var projects []*client.Project
	err = json.Unmarshal(b, &projects)
	if err != nil {
		log.Fatalf("failed to unmarshal fake data: %s", err)
	}

	return &fakeDependencyTrackClient{c, projects}
}

type fakeDependencyTrackClient struct {
	client.Client
	projects []*client.Project
}

func (f *fakeDependencyTrackClient) GetProjectsByTag(ctx context.Context, tag string) ([]*client.Project, error) {
	projects := make([]*client.Project, 0)
	value, err := url.QueryUnescape(tag)
	if err != nil {
		return nil, err
	}
	for _, p := range f.projects {
		for _, t := range p.Tags {
			if t.Name == value {
				projects = append(projects, p)
			}
		}
	}

	return projects, nil
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)
