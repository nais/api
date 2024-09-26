package vulnerabilities

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/google/uuid"
	"github.com/nais/dependencytrack/pkg/client"
)

func NewFakeDependencyTrackClient(c client.Client) client.Client {
	projects := createTestdata()
	return &fakeDependencyTrackClient{c, projects}
}

type fakeDependencyTrackClient struct {
	client.Client
	projects []*client.Project
}

func (f *fakeDependencyTrackClient) GetProject(_ context.Context, name, version string) (*client.Project, error) {
	for _, p := range f.projects {
		if p.Name == name {
			return p, nil
		}
	}
	return nil, nil
}

func (f *fakeDependencyTrackClient) GetFindings(ctx context.Context, projectUuid string, suppressed bool) ([]*client.Finding, error) {
	p, err := f.Client.GetProject(ctx, "ghcr.io/nais/testapp/testapp", "2020-02-25-f61e7b7")
	if err != nil {
		return nil, err
	}
	return f.Client.GetFindings(ctx, p.Uuid, suppressed)
}

func (f *fakeDependencyTrackClient) GetProjectsByTag(ctx context.Context, tag string) ([]*client.Project, error) {
	return f.projects, nil
}

func createTestdata() []*client.Project {
	projects := make([]*client.Project, 0)
	team := "devteam"
	for i := range 6 {
		p := createProject(team, "app", fmt.Sprintf("nais-deploy-chicken-%d", i+2), fmt.Sprintf("v%d", i+2), i)
		projects = append(projects, p)
	}
	projects = append(projects, createProject(team, "job", "dataproduct-apps-topics", fmt.Sprintf("v%d", 1), 4))
	projects = append(projects, createProject(team, "job", "dataproduct-naisjobs-topics", fmt.Sprintf("v%d", 1), 7))
	return projects
}

func createProject(team, workloadType, name, version string, vulnFactor int) *client.Project {
	p := &client.Project{
		Name:    "ghcr.io/nais/" + name,
		Tags:    make([]client.Tag, 0),
		Uuid:    uuid.New().String(),
		Version: version,
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
	p.Tags = append(p.Tags, client.Tag{Name: fmt.Sprintf("workload:%s|%s|%s|%s", "dev", team, workloadType, name)})
	p.Tags = append(p.Tags, client.Tag{Name: "env:" + "dev"})
	p.Tags = append(p.Tags, client.Tag{Name: fmt.Sprintf("workload:%s|%s|%s|%s", "superprod", team, workloadType, name)})
	p.Tags = append(p.Tags, client.Tag{Name: "env:" + "superprod"})
	return p
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)

type FakePrometheusClient struct {
	projects []*client.Project
}

var _ VulnerabilityPrometheus = &FakePrometheusClient{}

func NewFakePrometheusClient() VulnerabilityPrometheus {
	projects := createTestdata()
	return &FakePrometheusClient{projects: projects}
}

func (f FakePrometheusClient) Query(_ context.Context, query string, _ time.Time, _ ...promv1.Option) (model.Value, promv1.Warnings, error) {
	// Stores aggregated risk scores per team
	teamRiskScores := make(map[string]float64)

	for _, p := range f.projects {
		for _, tag := range p.Tags {
			if strings.HasPrefix(tag.Name, "team:") {
				team := strings.TrimPrefix(tag.Name, "team:")

				// Aggregate risk score for the team
				if strings.Contains(query, "slsa_workload_riskscore") {
					teamRiskScores[team] += p.Metrics.InheritedRiskScore
				}
			}
		}
	}

	val := model.Vector{}
	for teamName, riskScore := range teamRiskScores {
		val = append(val, &model.Sample{
			Metric: model.Metric{
				"workload_namespace": model.LabelValue(teamName),
			},
			Value:     model.SampleValue(riskScore),
			Timestamp: 1234567,
		})
	}

	// Sort the results by risk score (descending order)
	sort.Slice(val, func(i, j int) bool {
		return val[i].Value > val[j].Value
	})

	return val, nil, nil
}
