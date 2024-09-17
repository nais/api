package vulnerabilities

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/nais/dependencytrack/pkg/client"
	log "github.com/sirupsen/logrus"
	"net/url"
	"os"
	"strings"
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

	team := strings.Split(value, ":")[1]
	for i := range 6 {
		p := &client.Project{
			Name:    fmt.Sprintf("nais-deploy-chicken-%d", i+2),
			Tags:    make([]client.Tag, 0),
			Uuid:    uuid.New().String(),
			Version: uuid.New().String(),
			Metrics: &client.ProjectMetric{
				Critical:           i,
				High:               i * 2,
				Medium:             i + 2,
				Low:                i + 1,
				Unassigned:         i,
				FindingsTotal:      i + (i * 2) + (i + 2) + (i + 1) + i,
				InheritedRiskScore: float64(i*10 + (i*2)*5 + (i+2)*3 + (i + 1) + i*5),
				Components:         i + 1,
			},
			LastBomImport: 1,
		}

		workloadType := "app"
		if i%2 != 0 {
			workloadType = "job"
		}
		p.Tags = append(p.Tags, client.Tag{Name: "team:" + team})
		p.Tags = append(p.Tags, client.Tag{Name: "project:" + p.Name})
		p.Tags = append(p.Tags, client.Tag{Name: "image:" + p.Name + ":" + p.Version})
		p.Tags = append(p.Tags, client.Tag{Name: fmt.Sprintf("workload:%s|%s|%s|%s", "dev", team, workloadType, p.Name)})
		p.Tags = append(p.Tags, client.Tag{Name: "env:" + "dev"})
		p.Tags = append(p.Tags, client.Tag{Name: fmt.Sprintf("workload:%s|%s|%s|%s", "superprod", team, workloadType, p.Name)})
		p.Tags = append(p.Tags, client.Tag{Name: "env:" + "superprod"})

		projects = append(projects, p)
	}

	return projects, nil
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)
