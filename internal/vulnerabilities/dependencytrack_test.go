package vulnerabilities

import (
	"context"
	"testing"

	"github.com/nais/api/internal/graph/model"
	dependencytrack "github.com/nais/dependencytrack/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

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
				assert.Len(t, f, 2)
				assert.Equal(t, "CVE-2021-1234", f[1].Aliases[0].Name)
				assert.Equal(t, "NVD", f[1].Aliases[0].Source)
			},
		},
	}

	for _, tc := range tt {
		mock := NewMockInternalClient(t)
		c := NewDependencyTrackClient(DependencyTrackConfig{}, log, WithClient(mock))
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
