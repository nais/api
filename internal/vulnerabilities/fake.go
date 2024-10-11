package vulnerabilities

import (
	"context"
	"fmt"
	"math/rand"
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
	return &fakeDependencyTrackClient{c, projects, map[string]*client.Analysis{}}
}

type fakeDependencyTrackClient struct {
	client.Client
	projects      []*client.Project
	analysisTrail map[string]*client.Analysis
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
	for _, p := range f.projects {
		if p.Uuid == projectUuid {
			return createFindings(p), nil
		}
	}
	return nil, nil
}

func (f *fakeDependencyTrackClient) GetAnalysisTrail(ctx context.Context, projectUuid, componentUuid, vulnerabilityUuid string) (*client.Analysis, error) {
	return f.analysisTrail[projectUuid], nil
}

func (f *fakeDependencyTrackClient) RecordAnalysis(ctx context.Context, analysis *client.AnalysisRequest) error {
	a := &client.Analysis{
		AnalysisState:         analysis.AnalysisState,
		AnalysisJustification: analysis.AnalysisJustification,
		AnalysisResponse:      analysis.AnalysisResponse,
		AnalysisDetails:       analysis.AnalysisDetails,
		AnalysisComments: []client.AnalysisComment{
			{
				Timestamp: int(time.Now().Unix()),
				Comment:   analysis.Comment,
			},
		},
		IsSuppressed: analysis.IsSuppressed,
	}

	f.analysisTrail[analysis.Project] = a

	return nil
}

func (f *fakeDependencyTrackClient) TriggerAnalysis(ctx context.Context, projectUuid string) error {
	return nil
}

func (f *fakeDependencyTrackClient) GetProjectsByTag(ctx context.Context, tag string) ([]*client.Project, error) {
	return f.projects, nil
}

func createTestdata() []*client.Project {
	projects := make([]*client.Project, 0)
	team := "devteam"
	for i := range 6 {
		p := createProject(team, "app", fmt.Sprintf("nais-deploy-chicken-%d", i+2), "1", i)
		projects = append(projects, p)
	}
	projects = append(projects, createProject(team, "job", "dataproduct-apps-topics", fmt.Sprintf("v%d", 1), 4))
	projects = append(projects, createProject(team, "job", "dataproduct-naisjobs-topics", fmt.Sprintf("v%d", 1), 7))
	return projects
}

func createProject(team, workloadType, name, version string, vulnFactor int) *client.Project {
	pName := "ghcr.io/nais/" + name
	p := &client.Project{
		Name:    pName,
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

func createFindings(p *client.Project) []*client.Finding {
	findings := make([]*client.Finding, 0)

	for i := range p.Metrics.Critical {
		findings = append(findings, createFindingStruct(p.Uuid, 1, fmt.Sprintf("some-component-%d", i)))
	}
	for i := range p.Metrics.High {
		findings = append(findings, createFindingStruct(p.Uuid, 2, fmt.Sprintf("some-component-%d", i)))
	}
	for i := range p.Metrics.Medium {
		findings = append(findings, createFindingStruct(p.Uuid, 3, fmt.Sprintf("some-component-%d", i)))
	}
	for i := range p.Metrics.Low {
		findings = append(findings, createFindingStruct(p.Uuid, 4, fmt.Sprintf("some-component-%d", i)))
	}
	for i := range p.Metrics.Unassigned {
		findings = append(findings, createFindingStruct(p.Uuid, 5, fmt.Sprintf("some-component-%d", i)))
	}

	return findings
}

func createFindingStruct(projectId string, severity int, componentName string) *client.Finding {
	sName := ""
	switch severity {
	case 1:
		sName = "CRITICAL"
	case 2:
		sName = "HIGH"
	case 3:
		sName = "MEDIUM"
	case 4:
		sName = "LOW"
	case 5:
		sName = "UNASSIGNED"
	}

	return &client.Finding{
		Component: client.Component{
			UUID:    uuid.New().String(),
			PURL:    fmt.Sprintf("pkg:golang/%s@v2.0.8?type=module", componentName),
			Project: projectId,
			Name:    componentName,
		},
		Vulnerability: client.Vulnerability{
			UUID:         "31b633d1-557c-41e6-a7ab-b94a0b3e2c21",
			VulnId:       fmt.Sprintf("CVE-2024-%d", rand.Intn(100000)),
			Severity:     sName,
			SeverityRank: severity,
			Source:       "NVD",
			Title:        "",
		},
		Analysis: client.VulnzAnalysis{
			IsSuppressed: false,
			State:        "",
		},
	}
}

var _ client.Client = (*fakeDependencyTrackClient)(nil)

type FakePrometheusClient struct {
	projects []*client.Project
}

var _ Prometheus = &FakePrometheusClient{}

func NewFakePrometheusClient() Prometheus {
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
