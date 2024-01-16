package dependencytrack

import (
	"context"
	"net/url"
	"testing"

	"github.com/nais/api/internal/graph/model"
	dependencytrack "github.com/nais/dependencytrack/pkg/client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestClient_GetVulnerabilities(t *testing.T) {
	log := logrus.New().WithField("test", "dependencytrack")
	ctx := context.Background()

	defaultInput := []*AppInstance{
		{
			Env:   "dev",
			Team:  "team1",
			App:   "app1",
			Image: "image:latest",
		},
		{
			Env:   "dev",
			Team:  "team1",
			App:   "app2",
			Image: "image:latest",
		},
	}

	tt := []struct {
		name   string
		input  []*AppInstance
		expect func(input []*AppInstance, mock *MockInternalClient)
		assert func(t *testing.T, v []*model.Vulnerability, err error)
	}{
		{
			name:  "should return list with summary null if no apps have a project",
			input: defaultInput,
			expect: func(input []*AppInstance, mock *MockInternalClient) {
				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:latest")).Return([]*dependencytrack.Project{}, nil)
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
			input: []*AppInstance{
				{
					Env:   "dev",
					Team:  "team1",
					App:   "app1",
					Image: "image:latest",
				},
				{
					Env:   "dev",
					Team:  "team1",
					App:   "app2",
					Image: "image:notfound",
				},
			},
			expect: func(input []*AppInstance, mock *MockInternalClient) {
				p1 := project(input[0].ToTags()...)
				p1.LastBomImportFormat = "cyclonedx"

				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:latest")).Return([]*dependencytrack.Project{p1}, nil)
				mock.EXPECT().
					GetFindings(ctx, p1.Uuid).Return(findings(), nil)
				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:notfound")).Return([]*dependencytrack.Project{}, nil)
			},
			assert: func(t *testing.T, v []*model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.Len(t, v, 2)
				for _, vn := range v {
					if vn.AppName == "app1" {
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
			expect: func(input []*AppInstance, mock *MockInternalClient) {
				ps := make([]*dependencytrack.Project, 0)
				for _, i := range input {
					p := project(i.ToTags()...)
					p.LastBomImportFormat = "cyclonedx"
					ps = append(ps, p)
				}

				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:latest")).Return(ps, nil).Times(2)
				mock.EXPECT().
					GetFindings(ctx, ps[0].Uuid).Return(findings(), nil).Times(2)
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

func TestClient_VulnerabilitySummary(t *testing.T) {
	log := logrus.New().WithField("test", "dependencytrack")
	ctx := context.Background()

	tt := []struct {
		name   string
		input  *AppInstance
		expect func(input *AppInstance, mock *MockInternalClient)
		assert func(t *testing.T, v *model.Vulnerability, err error)
	}{
		{
			name:  "should return empty summary if no bom is found",
			input: app("dev", "team1", "app1", "image:latest"),
			expect: func(input *AppInstance, mock *MockInternalClient) {
				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:latest")).Return([]*dependencytrack.Project{project(input.ToTags()...)}, nil)
			},
			assert: func(t *testing.T, v *model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.Equal(t, -1, v.Summary.Critical)
				assert.Equal(t, -1, v.Summary.High)
				assert.Equal(t, -1, v.Summary.Medium)
				assert.Equal(t, -1, v.Summary.Low)
				assert.Equal(t, -1, v.Summary.Unassigned)
			},
		},
		{
			name:  "should return nil summary if no project is found",
			input: app("dev", "team1", "noProject", "image:latest"),
			expect: func(input *AppInstance, mock *MockInternalClient) {
				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:latest")).Return([]*dependencytrack.Project{}, nil)
			},
			assert: func(t *testing.T, v *model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.Nil(t, v.Summary)
			},
		},
		{
			name:  "should return summary with n vulnerabilities",
			input: app("dev", "team1", "app1", "image:latest"),
			expect: func(input *AppInstance, mock *MockInternalClient) {
				p := []*dependencytrack.Project{project(input.ToTags()...)}
				p[0].LastBomImportFormat = "cyclonedx"

				mock.EXPECT().
					GetProjectsByTag(ctx, url.QueryEscape("image:latest")).Return(p, nil)

				mock.EXPECT().
					GetFindings(ctx, p[0].Uuid).Return(findings(), nil)
			},
			assert: func(t *testing.T, v *model.Vulnerability, err error) {
				assert.NoError(t, err)
				assert.NotNilf(t, v.Summary, "summary is nil")
				assert.Equal(t, 4, v.Summary.Total)
				assert.Equal(t, 1, v.Summary.Critical)
				assert.Equal(t, 1, v.Summary.High)
				assert.Equal(t, 1, v.Summary.Medium)
				assert.Equal(t, 1, v.Summary.Low)
				assert.Equal(t, 0, v.Summary.Unassigned)
				// riskScore := (critical * 10) + (high * 5) + (medium * 3) + (low * 1) + (unassigned * 5)
				assert.Equal(t, 10+5+3+1, v.Summary.RiskScore)
			},
		},
	}

	for _, tc := range tt {
		mock := NewMockInternalClient(t)
		c := New("endpoint", "username", "password", "frontend", log).WithClient(mock)
		tc.expect(tc.input, mock)
		v, err := c.VulnerabilitySummary(ctx, tc.input)
		tc.assert(t, v, err)
	}
}

func app(env, team, app, image string) *AppInstance {
	return &AppInstance{
		Env:   env,
		Team:  team,
		App:   app,
		Image: image,
	}
}

func (a *AppInstance) ToTags() []string {
	return []string{a.Env, a.Team, a.App, a.Image}
}

func project(tags ...string) *dependencytrack.Project {
	p := &dependencytrack.Project{
		Uuid: "uuid",
		Name: "name",
		Tags: make([]dependencytrack.Tag, 0),
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
			},
		},
	}
}
