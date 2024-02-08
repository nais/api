package dependencytrack

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

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
	ProjectID            scalar.Ident
	VulnerabilitySummary *model.VulnerabilitySummary
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

	ch := cache.New(5*time.Minute, 10*time.Minute)

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

func (c *Client) GetProjectMetric(ctx context.Context, app *AppInstance) (*ProjectMetric, error) {
	p, err := c.retrieveProject(ctx, app)
	if err != nil {
		return nil, fmt.Errorf("getting project by app %s: %w", app.ID(), err)
	}
	if p == nil {
		return nil, nil
	}
	metric, err := c.client.GetCurrentProjectMetric(ctx, p.Uuid)
	if err != nil {
		if dependencytrack.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("getting current project metric: %w", err)
	}
	if metric == nil {
		return nil, nil
	}

	return &ProjectMetric{
		ProjectID: scalar.VulnerabilitiesIdent(p.Uuid),
		VulnerabilitySummary: &model.VulnerabilitySummary{
			Total:      metric.FindingsTotal,
			RiskScore:  metric.InheritedRiskScore,
			Critical:   metric.Critical,
			High:       metric.High,
			Medium:     metric.Medium,
			Low:        metric.Low,
			Unassigned: metric.Unassigned,
		},
	}, nil
}

func (c *Client) VulnerabilitySummary(ctx context.Context, app *AppInstance) (*model.Vulnerability, error) {
	return c.findingsForApp(ctx, app)
}

func (c *Client) GetVulnerabilities(ctx context.Context, apps []*AppInstance) ([]*model.Vulnerability, error) {
	var wg sync.WaitGroup
	now := time.Now()

	nodes := make([]*model.Vulnerability, 0)
	for _, a := range apps {
		wg.Add(1)
		go func(app *AppInstance) {
			defer wg.Done()
			v, err := c.findingsForApp(ctx, app)
			if err != nil {
				c.log.Errorf("retrieveFindings for app %q: %v", app.ID(), err)
				return
			}
			if v == nil {
				c.log.Debugf("no findings found in DependencyTrack for app %q", app.ID())
				return
			}
			nodes = append(nodes, v)
		}(a)
	}
	wg.Wait()

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
	v.HasBom = p.LastBomImportFormat != ""

	if !v.HasBom {
		c.log.Debugf("no bom found in DependencyTrack for project %s", p.Name)
		v.Summary = c.createSummary([]*dependencytrack.Finding{}, v.HasBom)
		c.cache.Set(app.ID(), v, cache.DefaultExpiration)
		return v, nil
	}

	f, err := c.retrieveFindings(ctx, p.Uuid)
	if err != nil {
		return nil, err
	}

	v.Summary = c.createSummary(f, v.HasBom)

	c.cache.Set(app.ID(), v, cache.DefaultExpiration)
	return v, nil
}

func (c *Client) retrieveFindings(ctx context.Context, uuid string) ([]*dependencytrack.Finding, error) {
	findings, err := c.client.GetFindings(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("retrieveFindings from DependencyTrack: %w", err)
	}

	return findings, nil
}

func (c *Client) createSummary(findings []*dependencytrack.Finding, hasBom bool) *model.VulnerabilitySummary {
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

	cves := make(map[string]*dependencytrack.Finding)
	for _, finding := range findings {
		cves[finding.Vulnerability.VulnId+":"+finding.Component.UUID] = finding
	}

	severities := map[string]int{}
	total := 0
	for _, finding := range findings {

		if finding.Vulnerability.Source == "NVD" {
			severities[finding.Vulnerability.Severity] += 1
			total++
			continue
		}

		if len(finding.Vulnerability.Aliases) == 0 {
			severities[finding.Vulnerability.Severity] += 1
			total++
		}

		for _, cve := range finding.Vulnerability.Aliases {
			nvdId := cve.CveId + ":" + finding.Component.UUID
			if _, found := cves[nvdId]; !found {
				severities[finding.Vulnerability.Severity] += 1
				total++
			}
		}
	}

	return &model.VulnerabilitySummary{
		Total:      total,
		RiskScore:  calcRiskScore(severities),
		Critical:   severities["CRITICAL"],
		High:       severities["HIGH"],
		Medium:     severities["MEDIUM"],
		Low:        severities["LOW"],
		Unassigned: severities["UNASSIGNED"],
	}
}

func calcRiskScore(severities map[string]int) int {
	// algorithm: https://github.com/DependencyTrack/dependency-track/blob/41e2ba8afb15477ff2b7b53bd9c19130ba1053c0/src/main/java/org/dependencytrack/metrics/Metrics.java#L31-L33
	return (severities["CRITICAL"] * 10) + (severities["HIGH"] * 5) + (severities["MEDIUM"] * 3) + (severities["LOW"] * 1) + (severities["UNASSIGNED"] * 5)
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
