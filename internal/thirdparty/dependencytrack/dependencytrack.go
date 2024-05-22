package dependencytrack

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sourcegraph/conc/pool"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	dependencytrack "github.com/nais/dependencytrack/pkg/client"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type AppInstance struct {
	Env, Team, App, Image string
}

func (a *AppInstance) ID() string {
	return fmt.Sprintf("%s:%s:%s:%s", a.Env, a.Team, a.App, a.Image)
}

func (a *AppInstance) ProjectName() string {
	return fmt.Sprintf("%s:%s:%s", a.Env, a.Team, a.App)
}

type ProjectMetric struct {
	ProjectID            uuid.UUID
	VulnerabilityMetrics []*VulnerabilityMetrics
}

type VulnerabilityMetrics struct {
	Total           int   `json:"total"`
	RiskScore       int   `json:"riskScore"`
	Critical        int   `json:"critical"`
	High            int   `json:"high"`
	Medium          int   `json:"medium"`
	Low             int   `json:"low"`
	Unassigned      int   `json:"unassigned"`
	FirstOccurrence int64 `json:"firstOccurrence"`
	LastOccurrence  int64 `json:"lastOccurrence"`
}

type Client struct {
	client      dependencytrack.Client
	frontendUrl string
	log         logrus.FieldLogger
	cache       *cache.Cache
}

func New(endpoint, username, password, frontend string, log *logrus.Entry) *Client {
	c := dependencytrack.New(
		endpoint,
		username,
		password,
		dependencytrack.WithApiKeySource("Administrators"),
		dependencytrack.WithLogger(log),
		dependencytrack.WithHttpClient(&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}),
	)

	ch := cache.New(30*time.Minute, 10*time.Minute)

	return &Client{
		client:      c,
		frontendUrl: frontend,
		log:         log,
		cache:       ch,
	}
}

func (c *Client) Init(ctx context.Context) error {
	_, err := c.client.Headers(ctx)
	if err != nil {
		return fmt.Errorf("initializing DependencyTrack client: %w", err)
	}
	return nil
}

func (c *Client) WithClient(client dependencytrack.Client) *Client {
	c.client = client
	return c
}

func (c *Client) WithCache(cache *cache.Cache) *Client {
	c.cache = cache
	return c
}

func (c *Client) GetProjectMetrics(ctx context.Context, app *AppInstance, date string) (*ProjectMetric, error) {
	p, err := c.retrieveProject(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("getting project by app %s: %w", app.ID(), err)
	}
	if p == nil {
		return nil, nil
	}
	metrics, err := c.client.GetProjectMetricsByDate(ctx, p.Uuid, date)
	if err != nil {
		if dependencytrack.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting current project metric: %w", err)
	}
	if metrics == nil {
		return nil, nil
	}

	vulnMetrics := make([]*VulnerabilityMetrics, len(metrics))
	for i, metric := range metrics {
		vulnMetrics[i] = &VulnerabilityMetrics{
			Total:           metric.FindingsTotal,
			RiskScore:       int(metric.InheritedRiskScore),
			Critical:        metric.Critical,
			High:            metric.High,
			Medium:          metric.Medium,
			Low:             metric.Low,
			Unassigned:      metric.Unassigned,
			FirstOccurrence: metric.FirstOccurrence,
			LastOccurrence:  metric.LastOccurrence,
		}
	}

	id, err := uuid.Parse(p.Uuid)
	if err != nil {
		return nil, fmt.Errorf("parsing project uuid: %w", err)
	}

	return &ProjectMetric{
		ProjectID:            id,
		VulnerabilityMetrics: vulnMetrics,
	}, nil
}

func (c *Client) VulnerabilitySummary(ctx context.Context, app *AppInstance) (*model.Vulnerability, error) {
	return c.findingsForApp(ctx, app)
}

type Filter = func(vulnerability *model.Vulnerability) bool

func RequireSbom() Filter {
	return func(vulnerability *model.Vulnerability) bool {
		return vulnerability.HasBom
	}
}

func (c *Client) GetVulnerabilities(ctx context.Context, apps []*AppInstance, filters ...Filter) ([]*model.Vulnerability, error) {
	now := time.Now()
	nodes := make([]*model.Vulnerability, 0)
	lock := sync.Mutex{}
	p := pool.New().WithMaxGoroutines(10)
	for _, app := range apps {
		p.Go(func() {
			appVulnNode, err := c.findingsForApp(ctx, app)
			if err != nil {
				c.log.Errorf("retrieveFindings for app %q: %v", app.ID(), err)
				return
			}
			if appVulnNode == nil {
				c.log.Debugf("no findings found in DependencyTrack for app %q", app.ID())
				return
			}
			for _, f := range filters {
				if !f(appVulnNode) {
					return
				}
			}
			lock.Lock()
			nodes = append(nodes, appVulnNode)
			lock.Unlock()
		})
	}
	p.Wait()
	c.log.Debugf("DependencyTrack fetch: %v\n", time.Since(now))
	return nodes, nil
}

func (c *Client) findingsForApp(ctx context.Context, app *AppInstance) (*model.Vulnerability, error) {
	if v, ok := c.cache.Get(app.ID()); ok {
		return v.(*model.Vulnerability), nil
	}

	v := &model.Vulnerability{
		ID:      scalar.VulnerabilitiesIdent(app.ID()),
		AppName: app.App,
		Env:     app.Env,
	}

	p, err := c.retrieveProject(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("getting project by app %s: %w", app.ID(), err)
	}
	if p == nil {
		return v, nil
	}

	u := strings.TrimSuffix(c.frontendUrl, "/")
	findingsLink := fmt.Sprintf("%s/projects/%s/findings", u, p.Uuid)

	v.FindingsLink = findingsLink
	v.HasBom = hasBom(p)

	if !v.HasBom {
		c.log.Debugf("no bom found in DependencyTrack for project %s", p.Name)
		v.Summary = c.createSummary(p, v.HasBom)
		c.cache.Set(app.ID(), v, cache.DefaultExpiration)
		return v, nil
	}

	v.Summary = c.createSummary(p, v.HasBom)

	c.cache.Set(app.ID(), v, cache.DefaultExpiration)
	return v, nil
}

// Due to the nature of the DependencyTrack API, the 'LastBomImportFormat' is not reliable to determine if a project has a BOM.
// The 'LastBomImportFormat' can be empty even if the project has a BOM.
// As a fallback, we can check if projects has registered any components, then we assume that if a project has components, it has a BOM.
func hasBom(p *dependencytrack.Project) bool {
	return p.LastBomImportFormat != "" || p.Metrics != nil && p.Metrics.Components > 0
}

func (c *Client) retrieveFindings(ctx context.Context, uuid string) ([]*dependencytrack.Finding, error) {
	findings, err := c.client.GetFindings(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("retrieveFindings from DependencyTrack: %w", err)
	}

	return findings, nil
}

func (c *Client) createSummary(project *dependencytrack.Project, hasBom bool) *model.VulnerabilitySummary {
	if !hasBom {
		return &model.VulnerabilitySummary{
			RiskScore:  -1,
			Total:      -1,
			Critical:   -1,
			High:       -1,
			Medium:     -1,
			Low:        -1,
			Unassigned: -1,
		}
	}

	return &model.VulnerabilitySummary{
		Total:      project.Metrics.FindingsTotal,
		RiskScore:  int(project.Metrics.InheritedRiskScore),
		Critical:   project.Metrics.Critical,
		High:       project.Metrics.High,
		Medium:     project.Metrics.Medium,
		Low:        project.Metrics.Low,
		Unassigned: project.Metrics.Unassigned,
	}
}

func (c *Client) retrieveProjectById(ctx context.Context, projectId string) (*dependencytrack.Project, error) {
	project, err := c.client.GetProjectById(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	return project, nil
}

func (c *Client) retrieveProjectsForTeam(ctx context.Context, team string) ([]*dependencytrack.Project, error) {
	tag := url.QueryEscape("team:" + team)
	if v, ok := c.cache.Get(tag); ok {
		return v.([]*dependencytrack.Project), nil
	}

	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	c.cache.Set(tag, projects, cache.DefaultExpiration)

	return projects, nil
}

func (c *Client) retrieveProject(ctx context.Context, app *AppInstance) (*dependencytrack.Project, error) {
	tag := url.QueryEscape(app.Image)
	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	if len(projects) == 0 {
		return nil, nil
	}
	var p *dependencytrack.Project
	for _, project := range projects {
		if containsAllTags(project.Tags, app.Env, app.Team, app.App) {
			p = project
			break
		}
	}
	return p, nil
}

func (c *Client) retrieveProjects(ctx context.Context, app *AppInstance) ([]*dependencytrack.Project, error) {
	tag := url.QueryEscape(app.Image)

	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	if len(projects) == 0 {
		return nil, nil
	}
	return projects, nil
}

func (c *Client) GetFindingsForImageByProjectID(ctx context.Context, projectId string) ([]*model.Finding, error) {
	findings, err := c.retrieveFindings(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("retrieving findings for project %s: %w", projectId, err)
	}

	retFindings := make([]*model.Finding, 0)
	for _, f := range findings {
		if f.Vulnerability.Severity == "UNASSIGNED" {
			continue
		}
		cveId := ""
		ghsaId := ""
		osvId := ""

		for _, alias := range f.Vulnerability.Aliases {
			cveId = alias.CveId
			ghsaId = alias.GhsaId
		}

		if f.Vulnerability.Source == "OSV" {
			osvId = f.Vulnerability.VulnId
		}

		retFindings = append(retFindings, &model.Finding{
			ID:              scalar.FindingIdent(f.Vulnerability.VulnId),
			ComponentID:     f.Component.UUID,
			Severity:        f.Vulnerability.Severity,
			Description:     f.Vulnerability.Title,
			CveID:           cveId,
			GhsaID:          ghsaId,
			OsvID:           osvId,
			PackageURL:      f.Component.PURL,
			VulnerabilityID: f.Vulnerability.VulnId,
		})
	}
	return retFindings, nil
}

func (c *Client) GetMetadataForImageByProjectID(ctx context.Context, projectId string) (*model.Image, error) {
	p, err := c.retrieveProjectById(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("getting project by id %s: %w", projectId, err)
	}

	if p == nil {
		return nil, fmt.Errorf("project not found: %s", projectId)
	}

	var digest string
	var rekor string
	var workloads []*model.WorkloadReference
	for _, tag := range p.Tags {
		if strings.Contains(tag.Name, "digest:") {
			digest = strings.TrimPrefix(tag.Name, "digest:")
		}
		if strings.Contains(tag.Name, "rekor:") {
			rekor = strings.TrimPrefix(tag.Name, "rekor:")
		}
		if strings.Contains(tag.Name, "workload:") {
			w := strings.TrimPrefix(tag.Name, "workload:")
			workload := strings.Split(w, "|")

			workloads = append(workloads, &model.WorkloadReference{
				ID:           scalar.WorkloadIdent(w),
				Environment:  workload[0],
				Team:         workload[1],
				WorkloadType: workload[2],
				Name:         workload[3]})

		}
	}

	summary := c.createSummary(p, true)

	return &model.Image{
		Name:      p.Name + ":" + p.Version,
		ID:        scalar.ImageIdent(p.Name),
		Digest:    digest,
		RekorID:   rekor,
		Version:   p.Version,
		ProjectID: p.Uuid,
		Summary: model.VulnerabilitySummary{
			Total:      summary.Total,
			Critical:   summary.Critical,
			RiskScore:  summary.RiskScore,
			High:       summary.High,
			Medium:     summary.Medium,
			Low:        summary.Low,
			Unassigned: summary.Unassigned,
		},
		WorkloadReferences: workloads,
	}, nil
}

func (c *Client) GetMetadataForTeam(ctx context.Context, team string) ([]*model.Image, error) {
	projects, err := c.retrieveProjectsForTeam(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("getting projects by team %s: %w", team, err)
	}

	if projects == nil {
		return nil, nil
	}

	images := make([]*model.Image, 0)

	for _, project := range projects {
		if project == nil {
			continue
		}
		if strings.Contains(project.Name, "nais-io") {
			continue
		}

		var digest string
		var rekor string
		var version string
		var workloads []*model.WorkloadReference

		for _, tag := range project.Tags {
			if strings.Contains(tag.Name, "digest:") {
				digest = strings.TrimPrefix(tag.Name, "digest:")
			}
			if strings.Contains(tag.Name, "rekor:") {
				rekor = strings.TrimPrefix(tag.Name, "rekor:")
			}
			if strings.Contains(tag.Name, "version:") {
				version = strings.TrimPrefix(tag.Name, "version:")
			}
			if strings.Contains(tag.Name, "workload:") {
				w := strings.TrimPrefix(tag.Name, "workload:")
				workload := strings.Split(w, "|")

				workloads = append(workloads, &model.WorkloadReference{
					ID:           scalar.WorkloadIdent(w),
					Environment:  workload[0],
					Team:         workload[1],
					WorkloadType: workload[2],
					Name:         workload[3]})
			}
		}

		summary := c.createSummary(project, true)

		image := &model.Image{
			ID:                 scalar.DependencyTrackProjectIdent(project.Uuid),
			ProjectID:          project.Uuid,
			Name:               project.Name,
			Summary:            *summary,
			Digest:             digest,
			RekorID:            rekor,
			Version:            version,
			WorkloadReferences: workloads,
		}
		images = append(images, image)

	}

	return images, nil
}

/*
	func (c *Client) GetFindingsForImage(ctx context.Context, app *AppInstance) (*model.Image, error) {
		projects, err := c.retrieveProjects(ctx, app) // 4 prosjekter
		if err != nil {
			return nil, fmt.Errorf("getting project by app %s: %w", app.ID(), err)
		}

		if projects == nil {
			return nil, nil
		}

		// Finds index of project with latest bom import
		var lastBomImport int64
		var projectIndex int
		for i, project := range projects {
			if project.LastBomImport > lastBomImport {
				lastBomImport = project.LastBomImport
				projectIndex = i
			}
		}

		var digest string
		var rekor string
		for _, tag := range projects[projectIndex].Tags {
			if strings.Contains(tag.Name, "digest:") {
				digest = strings.TrimPrefix(tag.Name, "digest:")
			}
			if strings.Contains(tag.Name, "rekor:") {
				rekor = strings.TrimPrefix(tag.Name, "rekor:")
			}
		}

		findings, err := c.retrieveFindings(ctx, projects[projectIndex].Uuid)
		if err != nil {
			return nil, fmt.Errorf("retrieving findings for project %s: %w", projects[projectIndex].Uuid, err)
		}

		retFindings := make([]*model.Finding, 0)
		for _, f := range findings {
			if f.Vulnerability.Severity == "UNASSIGNED" {
				continue
			}
			cveId := ""
			ghsaId := ""
			osvId := ""

			for _, alias := range f.Vulnerability.Aliases {
				cveId = alias.CveId
				ghsaId = alias.GhsaId
			}

			if f.Vulnerability.Source == "OSV" {
				osvId = f.Vulnerability.VulnId
			}

			retFindings = append(retFindings, &model.Finding{
				ID:              scalar.FindingIdent(f.Vulnerability.VulnId),
				ComponentID:     f.Component.UUID,
				Severity:        f.Vulnerability.Severity,
				Description:     f.Vulnerability.Title,
				CveID:           cveId,
				GhsaID:          ghsaId,
				OsvID:           osvId,
				PackageURL:      f.Component.PURL,
				VulnerabilityID: f.Vulnerability.VulnId,
			})
		}

		summary := c.createSummary(projects[projectIndex], true)

		return &model.Image{
			Findings: model.FindingList{
				Nodes: retFindings,
				PageInfo: model.PageInfo{
					TotalCount: len(retFindings),
				},
			},
			Digest:  digest,
			RekorID: rekor,
			Summary: *summary,
			Name:    app.Image,
			ID:      scalar.ImageIdent(app.Image),
		}, nil
	}
*/
func containsAllTags(tags []dependencytrack.Tag, s ...string) bool {
	found := 0
	for _, t := range s {
		for _, tag := range tags {
			if tag.Name == t {
				found += 1
				break
			}
		}
	}
	return found == len(s)
}
