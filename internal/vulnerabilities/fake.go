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
		p := createProject(team, "app", fmt.Sprintf("nais-deploy-chicken-%d", i+2), i)
		projects = append(projects, p)
	}
	projects = append(projects, createProject(team, "job", "dataproduct-apps-topics", 4))
	projects = append(projects, createProject(team, "job", "dataproduct-naisjobs-topics", 7))

	return projects, nil
}

func createProject(team, workloadType, name string, vulnFactor int) *client.Project {
	p := &client.Project{
		Name:    name,
		Tags:    make([]client.Tag, 0),
		Uuid:    uuid.New().String(),
		Version: uuid.New().String(),
		Metrics: &client.ProjectMetric{
			Critical:           vulnFactor,
			High:               vulnFactor * 2,
			Medium:             vulnFactor + 2,
			Low:                vulnFactor + 1,
			Unassigned:         vulnFactor,
			FindingsTotal:      vulnFactor + (vulnFactor * 2) + (vulnFactor + 2) + (vulnFactor + 1) + vulnFactor,
			InheritedRiskScore: float64(vulnFactor*10 + (vulnFactor*2)*5 + (vulnFactor+2)*3 + (vulnFactor + 1) + vulnFactor*5),
			Components:         vulnFactor + 1,
		},
		LastBomImport: 1,
	}

	p.Tags = append(p.Tags, client.Tag{Name: "team:" + team})
	p.Tags = append(p.Tags, client.Tag{Name: "project:" + p.Name})
	p.Tags = append(p.Tags, client.Tag{Name: "image:" + p.Name + ":" + p.Version})
	p.Tags = append(p.Tags, client.Tag{Name: fmt.Sprintf("workload:%s|%s|%s|%s", "dev", team, workloadType, p.Name)})
	p.Tags = append(p.Tags, client.Tag{Name: "env:" + "dev"})
	p.Tags = append(p.Tags, client.Tag{Name: fmt.Sprintf("workload:%s|%s|%s|%s", "superprod", team, workloadType, p.Name)})
	p.Tags = append(p.Tags, client.Tag{Name: "env:" + "superprod"})
	return p
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)
