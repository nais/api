package vulnerabilities

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/dependencytrack/pkg/client"
	prom_model "github.com/prometheus/common/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestManager_GetSummaryForTeam(t *testing.T) {
	md := NewMockInternalClient(t)
	mp := NewMockPrometheus(t)
	m := setupManager(md, mp)
	ctx := context.Background()

	tt := []struct {
		name         string
		expectations func()
		workloads    []model.Workload
		assertions   func(summary *model.VulnerabilitySummaryForTeam)
	}{
		{
			name: "team summary should be summarized correctly",
			expectations: func() {
				md.EXPECT().GetProjectsByTag(ctx, "team%3Ateam1").Return([]*client.Project{
					testProject("team1", "app", "app1", "latest", 1, 2, 3, 4, 5),
					testProject("team1", "app", "app2", "latest", 1, 2, 3, 4, 5),
					testProject("team1", "app", "app3", "latest", 1, 2, 3, 4, 5),
				}, nil)

				mp.EXPECT().Query(ctx, "sum(slsa_workload_riskscore) by (workload_namespace)", mock.Anything).Return(
					promRiskScores(map[string]float64{
						"team1": 174,
						"team2": 250,
						"team3": 25,
					}), nil, nil)
				mp.EXPECT().Query(ctx, `sum(slsa_workload_riskscore{workload_namespace="team1"})`, mock.Anything).Return(
					promRiskScores(map[string]float64{
						"team1": 174,
					}), nil, nil)
			},
			workloads: []model.Workload{
				createWorkload("dev", "app1"),
				createWorkload("dev", "app2"),
				createWorkload("dev", "app3"),
				createWorkload("dev", "app4"),
			},
			assertions: func(s *model.VulnerabilitySummaryForTeam) {
				assert.Equal(t, 4, s.TotalWorkloads)
				assert.Equal(t, 3, s.Critical)
				assert.Equal(t, 6, s.High)
				assert.Equal(t, 9, s.Medium)
				assert.Equal(t, 12, s.Low)
				assert.Equal(t, 15, s.Unassigned)
				assert.Equal(t, calcRiskscore(3, 6, 9, 12, 15), s.RiskScore)
				assert.Equal(t, 3, s.BomCount)
				assert.Equal(t, 75.0, s.Coverage)
				states := make([]model.VulnerabilityState, 0)
				for _, st := range s.Status {
					states = append(states, st.State)
				}
				assert.Contains(t, states, model.VulnerabilityStateCoverageTooLow, model.VulnerabilityStateTooManyVulnerableWorkloads)
				assert.Equal(t, model.VulnerabilityRankingMiddle, s.VulnerabilityRanking)
			},
		},
	}

	for _, tc := range tt {
		tc.expectations()
		summary, err := m.GetSummaryForTeam(ctx, tc.workloads, "team1", 5)
		assert.NoError(t, err)
		tc.assertions(summary)
	}
}

func TestManager_getVulnerabilityScore(t *testing.T) {
	md := NewMockInternalClient(t)
	mp := NewMockPrometheus(t)
	m := setupManager(md, mp)
	ctx := context.Background()

	mp.EXPECT().Query(ctx, "sum(slsa_workload_riskscore) by (workload_namespace)", mock.Anything).Return(
		promRiskScores(map[string]float64{
			"team1": 1,
			"team2": 2,
			"team3": 3,
		}), nil, nil)

	tt := []struct {
		name         string
		team         string
		expectations func()
		assertions   func(rank model.VulnerabilityRanking)
	}{
		{
			name: "team is in the most vulnerable percentile",
			team: "team3",
			assertions: func(rank model.VulnerabilityRanking) {
				assert.Equal(t, model.VulnerabilityRankingMostVulnerable, rank)
			},
		},
		{
			name: "team is in the least vulnerable percentile",
			team: "team1",
			assertions: func(rank model.VulnerabilityRanking) {
				assert.Equal(t, model.VulnerabilityRankingLeastVulnerable, rank)
			},
		},
		{
			name: "team is in the middle percentile",
			team: "team2",
			assertions: func(rank model.VulnerabilityRanking) {
				assert.Equal(t, model.VulnerabilityRankingMiddle, rank)
			},
		},
		{
			name: "team ranking is unknown",
			team: "teamUnknown",
			assertions: func(rank model.VulnerabilityRanking) {
				assert.Equal(t, model.VulnerabilityRankingUnknown, rank)
			},
		},
	}

	for _, tc := range tt {
		rank, err := m.getVulnerabilityRanking(ctx, tc.team, 3)
		assert.NoError(t, err)
		tc.assertions(rank)
	}
}

func setupManager(md *MockInternalClient, mp *MockPrometheus) *Manager {
	log := logrus.New().WithField("test", "vulnerabilities")

	c := NewDependencyTrackClient(DependencyTrackConfig{}, log, WithClient(md))

	pc := &PrometheusClients{
		cfg:     &PrometheusConfig{Clusters: []string{"test"}},
		clients: map[string]Prometheus{"test": mp},
	}
	m := NewManager(
		&Config{Prometheus: PrometheusConfig{Clusters: []string{"test"}}},
		WithDependencyTrackClient(c),
		WithPrometheusClients(pc),
	)
	return m
}

func promRiskScores(scores map[string]float64) prom_model.Vector {
	val := prom_model.Vector{}
	for teamName, riskScore := range scores {
		val = append(val, &prom_model.Sample{
			Metric: prom_model.Metric{
				"workload_namespace": prom_model.LabelValue(teamName),
			},
			Value:     prom_model.SampleValue(riskScore),
			Timestamp: 1234567,
		})
	}

	// Sort the results by risk score (descending order)
	sort.Slice(val, func(i, j int) bool {
		return val[i].Value > val[j].Value
	})

	return val
}

func createWorkload(env, name string) model.Workload {
	return &model.App{
		WorkloadBase: model.WorkloadBase{
			Name: name,
			Env:  model.Env{Name: env},
		},
	}
}

func testProject(team, workloadType, name, version string, severities ...int) *client.Project {
	p := &client.Project{
		Name:          "ghcr.io/nais/" + name,
		Tags:          make([]client.Tag, 0),
		Uuid:          uuid.New().String(),
		Version:       version,
		Metrics:       metrics(severities...),
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

func metrics(vuln ...int) *client.ProjectMetric {
	m := &client.ProjectMetric{}
	for i, v := range vuln {
		switch i {
		case 0:
			m.Critical = v
		case 1:
			m.High = v
		case 2:
			m.Medium = v
		case 3:
			m.Low = v
		case 4:
			m.Unassigned = v
		}
	}

	m.FindingsTotal = m.Critical + m.High + m.Medium + m.Low + m.Unassigned
	m.InheritedRiskScore = float64(calcRiskscore(m.Critical, m.High, m.Medium, m.Low, m.Unassigned))
	m.Components = 1
	return m
}

func calcRiskscore(c, h, m, l, u int) int {
	return c*10 + h*5 + m*3 + l + u*5
}
