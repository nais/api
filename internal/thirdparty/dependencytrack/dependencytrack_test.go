package dependencytrack

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/nais/api/internal/graph/model"
	dependencytrack "github.com/nais/dependencytrack/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestClient_GetVulnerabilities(t *testing.T) {
	log := logrus.New().WithField("test", "dependencytrack")
	ctx := context.Background()

	defaultInput := []*WorkloadInstance{
		{
			Env:   "dev",
			Team:  "team1",
			Name:  "app1",
			Image: "test/image:latest",
		},
		{
			Env:   "dev",
			Team:  "team1",
			Name:  "app2",
			Image: "test/image:latest",
		},
	}

	tt := []struct {
		name   string
		input  []*WorkloadInstance
		expect func(input []*WorkloadInstance, mock *MockInternalClient)
		assert func(t *testing.T, v []*model.Vulnerability, err error)
	}{
		{
			name:  "should return list with summary null if no apps have a project",
			input: defaultInput,
			expect: func(input []*WorkloadInstance, mock *MockInternalClient) {
				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:test/image:latest")).Return([]*dependencytrack.Project{}, nil)
			},
			assert: func(t *testing.T, v []*model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.Len(t, v, 2)
				assert.Nil(t, v[0].Summary)
				assert.Nil(t, v[1].Summary)
			},
		},
		{
			name: "list of appinstance should be equal lenght to list of vulnerabilities even though some apps have no project",
			input: []*WorkloadInstance{
				{
					Env:   "dev",
					Team:  "team1",
					Name:  "app1",
					Image: "test/image:latest",
				},
				{
					Env:   "env:dev",
					Team:  "team:team1",
					Name:  "app2",
					Image: "test/image:notfound",
				},
			},
			expect: func(input []*WorkloadInstance, mock *MockInternalClient) {
				metrics := &dependencytrack.ProjectMetric{
					Critical:      1,
					High:          1,
					Medium:        1,
					Low:           1,
					Unassigned:    0,
					FindingsTotal: 4,
				}
				p1 := project(metrics, input[0].ToTags()...)
				p1.LastBomImportFormat = "cyclonedx"

				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:test/image:latest")).Return([]*dependencytrack.Project{p1}, nil)
				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:test/image:notfound")).Return([]*dependencytrack.Project{}, nil)
			},
			assert: func(t *testing.T, v []*model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.Len(t, v, 2)
				for _, vn := range v {
					if vn.AppName == "app1" {
						fmt.Println(vn.Summary)
						assert.NotNil(t, vn.Summary)
					}
					if vn.AppName == "app2" {
						assert.Nil(t, vn.Summary)
					}
				}
			},
		},
		{
			name:  "should return list with summaries if apps have a project",
			input: defaultInput,
			expect: func(input []*WorkloadInstance, mock *MockInternalClient) {
				ps := make([]*dependencytrack.Project, 0)
				for _, i := range input {
					metrics := &dependencytrack.ProjectMetric{
						Critical:      1,
						High:          1,
						Medium:        1,
						Low:           1,
						Unassigned:    0,
						FindingsTotal: 4,
					}
					p := project(metrics, i.ToTags()...)
					p.LastBomImportFormat = "cyclonedx"
					ps = append(ps, p)
				}

				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:test/image:latest")).Return(ps, nil).Times(2)
			},
			assert: func(t *testing.T, v []*model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.Equal(t, 2, len(v))
				assert.NotNil(t, v[0].Summary)
				assert.NotNil(t, v[1].Summary)
				assert.Equal(t, 4, v[0].Summary.Total)
				assert.Equal(t, 4, v[1].Summary.Total)
			},
		},
	}

	for _, tc := range tt {
		mock := NewMockInternalClient(t)
		c := New("endpoint", "username", "password", "frontend", log).WithClient(mock)
		tc.expect(tc.input, mock)
		v, err := c.GetVulnerabilities(ctx, tc.input)
		tc.assert(t, v, err)
	}
}

func TestClient_CreateSummaryForTeam(t *testing.T) {
	log := logrus.New().WithField("test", "dependencytrack")
	mock := NewMockInternalClient(t)
	c := New("endpoint", "username", "password", "frontend", log).WithClient(mock)

	s, err := os.ReadFile("testdata/tpsws.json")
	assert.NoError(t, err)
	var p *dependencytrack.Project
	err = json.Unmarshal(s, &p)
	assert.NoError(t, err)
	sum := c.createSummaryForTeam(p, true)
	assert.Equal(t, 218, sum.Total)
	assert.Equal(t, 42, sum.Critical)
	assert.Equal(t, 111, sum.High)
	assert.Equal(t, 58, sum.Medium)
	assert.Equal(t, 7, sum.Low)
	assert.Equal(t, 0, sum.Unassigned)
}

func TestClient_GetFindingsForImage(t *testing.T) {
	log := logrus.New().WithField("test", "dependencytrack")
	ctx := context.Background()

	tt := []struct {
		name   string
		input  *WorkloadInstance
		expect func(input *WorkloadInstance, mock *MockInternalClient)
		assert func(t *testing.T, f []*model.Finding, err error)
	}{
		{
			name:  "should return findings if project is found",
			input: workloadInstance("dev", "team1", "app1", "image:latest"),
			expect: func(input *WorkloadInstance, mock *MockInternalClient) {
				p := project(&dependencytrack.ProjectMetric{}, input.ToTags()...)
				p.LastBomImportFormat = "cyclonedx"

				mock.EXPECT().
					GetFindings(ctx, p.Uuid, false).Return(findings(), nil)
			},
			assert: func(t *testing.T, f []*model.Finding, err error) {
				assert.NoError(t, err)
				assert.Len(t, f, 4)
				assert.Equal(t, "CVE-2021-1234", f[3].Aliases[0].Name)
				assert.Equal(t, "NVD", f[3].Aliases[0].Source)
				assert.Equal(t, "GHSA-2021-1234", f[3].Aliases[1].Name)
				assert.Equal(t, "GHSA", f[3].Aliases[1].Source)
			},
		},
	}

	for _, tc := range tt {
		mock := NewMockInternalClient(t)
		c := New("endpoint", "username", "password", "frontend", log).WithClient(mock)
		tc.expect(tc.input, mock)
		f, err := c.GetFindingsForImageByProjectID(ctx, "uuid", false)
		tc.assert(t, f, err)

	}
}

func workloadInstance(env, team, app, image string) *WorkloadInstance {
	return &WorkloadInstance{
		Env:   env,
		Team:  team,
		Name:  app,
		Image: image,
		Kind:  "app",
	}
}

func (a *WorkloadInstance) ToTags() []string {
	return []string{
		dependencytrack.EnvironmentTagPrefix.With(a.Env),
		dependencytrack.TeamTagPrefix.With(a.Team),
		dependencytrack.WorkloadTagPrefix.With(a.Env + "|" + a.Team + "|" + a.Kind + "|" + a.Name),
		dependencytrack.ImageTagPrefix.With(a.Image),
	}
}

func project(metrics *dependencytrack.ProjectMetric, tags ...string) *dependencytrack.Project {
	p := &dependencytrack.Project{
		Uuid:    "uuid",
		Name:    "name",
		Tags:    make([]dependencytrack.Tag, 0),
		Metrics: metrics,
	}
	for _, tag := range tags {
		p.Tags = append(p.Tags, dependencytrack.Tag{Name: tag})
	}
	return p
}

func findings() []*dependencytrack.Finding {
	return []*dependencytrack.Finding{
		{
			Vulnerability: dependencytrack.Vulnerability{
				Severity: "LOW",
			},
		},
		{
			Vulnerability: dependencytrack.Vulnerability{
				Severity: "MEDIUM",
			},
		},
		{
			Vulnerability: dependencytrack.Vulnerability{
				Severity: "HIGH",
			},
		},
		{
			Vulnerability: dependencytrack.Vulnerability{
				Severity: "CRITICAL",
				VulnId:   "4",
				Title:    "title4",
				Aliases: []dependencytrack.Alias{
					{
						CveId:  "CVE-2021-1234",
						GhsaId: "GHSA-2021-1234",
					},
				},
			},
		},
	}
}
