package dependencytrack

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
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

type WorkloadInstance struct {
	Env, Team, Name, Image, Kind string
}

func (a *WorkloadInstance) ID() string {
	return fmt.Sprintf("%s:%s:%s:%s:%s", a.Env, a.Team, a.Name, a.Kind, a.Image)
}

func (a *WorkloadInstance) ProjectName() string {
	return fmt.Sprintf("%s:%s:%s", a.Env, a.Team, a.Name)
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

func (c *Client) GetProjectMetrics(ctx context.Context, instance *WorkloadInstance, date string) (*ProjectMetric, error) {
	p, err := c.retrieveProject(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("getting project by workload %s: %w", instance.ID(), err)
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

type Filter = func(vulnerability *model.Vulnerability) bool

func RequireSbom() Filter {
	return func(vulnerability *model.Vulnerability) bool {
		return vulnerability.HasBom
	}
}

func (c *Client) GetVulnerabilities(ctx context.Context, instances []*WorkloadInstance, filters ...Filter) ([]*model.Vulnerability, error) {
	now := time.Now()
	nodes := make([]*model.Vulnerability, 0)
	lock := sync.Mutex{}
	p := pool.New().WithMaxGoroutines(10)
	for _, instance := range instances {
		p.Go(func() {
			instanceVulnNode, err := c.findingsForWorkload(ctx, instance)
			if err != nil {
				c.log.Errorf("retrieveFindings for workload %q: %v", instance.ID(), err)
				return
			}
			if instanceVulnNode == nil {
				c.log.Debugf("no findings found in DependencyTrack for workload %q", instance.ID())
				return
			}
			for _, f := range filters {
				if !f(instanceVulnNode) {
					return
				}
			}
			lock.Lock()
			nodes = append(nodes, instanceVulnNode)
			lock.Unlock()
		})
	}
	p.Wait()
	c.log.Debugf("DependencyTrack fetch: %v\n", time.Since(now))
	return nodes, nil
}

func (c *Client) findingsForWorkload(ctx context.Context, instance *WorkloadInstance) (*model.Vulnerability, error) {
	if v, ok := c.cache.Get(instance.ID()); ok {
		return v.(*model.Vulnerability), nil
	}

	v := &model.Vulnerability{
		ID:      scalar.VulnerabilitiesIdent(instance.ID()),
		AppName: instance.Name,
		Env:     instance.Env,
	}

	p, err := c.retrieveProject(ctx, instance)
	if err != nil {
		return nil, fmt.Errorf("getting project by instance %s: %w", instance.ID(), err)
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
		v.Summary = c.createSummaryForTeam(p, v.HasBom)
		c.cache.Set(instance.ID(), v, cache.DefaultExpiration)
		return v, nil
	}

	v.Summary = c.createSummaryForTeam(p, v.HasBom)

	c.cache.Set(instance.ID(), v, cache.DefaultExpiration)
	return v, nil
}

// Due to the nature of the DependencyTrack API, the 'LastBomImportFormat' is not reliable to determine if a project has a BOM.
// The 'LastBomImportFormat' can be empty even if the project has a BOM.
// As a fallback, we can check if projects has registered any components, then we assume that if a project has components, it has a BOM.
func hasBom(p *dependencytrack.Project) bool {
	return p.LastBomImportFormat != "" || p.Metrics != nil && p.Metrics.Components > 0
}

func (c *Client) retrieveFindings(ctx context.Context, uuid string, suppressed bool) ([]*dependencytrack.Finding, error) {
	findings, err := c.client.GetFindings(ctx, uuid, suppressed)
	if err != nil {
		return nil, fmt.Errorf("retrieveFindings from DependencyTrack: %w", err)
	}

	return findings, nil
}

func (c *Client) createSummaryForTeam(project *dependencytrack.Project, hasBom bool) *model.VulnerabilitySummaryForTeam {
	if !hasBom || project.Metrics == nil {
		return nil
	}

	return &model.VulnerabilitySummaryForTeam{
		Total:      project.Metrics.FindingsTotal,
		RiskScore:  int(project.Metrics.InheritedRiskScore),
		Critical:   project.Metrics.Critical,
		High:       project.Metrics.High,
		Medium:     project.Metrics.Medium,
		Low:        project.Metrics.Low,
		Unassigned: project.Metrics.Unassigned,
		BomCount:   1,
	}
}

func (c *Client) createSummaryForImage(project *dependencytrack.Project, hasBom bool) *model.ImageVulnerabilitySummary {
	if !hasBom || project.Metrics == nil {
		return nil
	}

	return &model.ImageVulnerabilitySummary{
		ID:         scalar.ImageVulnerabilitySummaryIdent(project.Uuid),
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

func (c *Client) retrieveProject(ctx context.Context, instance *WorkloadInstance) (*dependencytrack.Project, error) {
	instanceImageTag := dependencytrack.ImageTagPrefix.With(instance.Image)
	tag := url.QueryEscape(instanceImageTag)
	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	if len(projects) == 0 {
		return nil, nil
	}

	var p *dependencytrack.Project
	for _, project := range projects {
		if containsAllTags(project.Tags, instanceImageTag) {
			p = project
			break
		}
	}
	return p, nil
}

func (c *Client) GetMetadataForImage(ctx context.Context, image string) (*model.ImageDetails, error) {
	name, version, _ := strings.Cut(image, ":")

	p, err := c.client.GetProject(ctx, name, version)
	if err != nil {
		return nil, fmt.Errorf("getting project by name %s and version %s: %w", name, version, err)
	}

	if p == nil {
		return &model.ImageDetails{
			ID:      scalar.ImageIdent(name, version),
			Name:    image,
			Version: version,
			Summary: c.createSummaryForImage(nil, false),
			Rekor:   model.Rekor{},
		}, nil
	}

	return &model.ImageDetails{
		Name:       p.Name + ":" + p.Version,
		ID:         scalar.ImageIdent(p.Name, p.Version),
		Rekor:      parseRekorTags(p.Tags),
		Version:    p.Version,
		HasSbom:    hasBom(p),
		ProjectID:  p.Uuid,
		Summary:    c.createSummaryForImage(p, hasBom(p)),
		ProjectURL: c.frontendUrl + "/projects/" + p.Uuid,
		GQLVars: model.ImageDetailsGQLVars{
			WorkloadReferences: parseWorkloadRefTags(p.Tags),
		},
	}, nil
}

func parseRekorTags(tags []dependencytrack.Tag) model.Rekor {
	var rekor model.Rekor
	for _, tag := range tags {
		switch {
		case strings.Contains(tag.Name, dependencytrack.RekorBuildConfigURITagPrefix.String()):
			rekor.BuildConfigURI = tag.Name
		case strings.Contains(tag.Name, dependencytrack.RekorGitHubWorkflowSHATagPrefix.String()):
			rekor.GitHubWorkflowSha = strings.TrimPrefix(tag.Name, dependencytrack.RekorGitHubWorkflowSHATagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorGitHubWorkflowNameTagPrefix.String()):
			rekor.GitHubWorkflowName = strings.TrimPrefix(tag.Name, dependencytrack.RekorGitHubWorkflowNameTagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorGitHubWorkflowRefTagPrefix.String()):
			rekor.GitHubWorkflowRef = strings.TrimPrefix(tag.Name, dependencytrack.RekorGitHubWorkflowRefTagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorIntegratedTimeTagPrefix.String()):
			trimedIntegratedTime := strings.TrimPrefix(tag.Name, dependencytrack.RekorIntegratedTimeTagPrefix.String())
			// parse string to int
			if integratedTime, err := strconv.ParseInt(trimedIntegratedTime, 10, 64); err == nil {
				rekor.IntegratedTime = int(integratedTime)
			} else {
				rekor.IntegratedTime = 0
			}
		case strings.Contains(tag.Name, dependencytrack.RekorOIDCIssuerTagPrefix.String()):
			rekor.OIDCIssuer = strings.TrimPrefix(tag.Name, dependencytrack.RekorOIDCIssuerTagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorRunInvocationURITagPrefix.String()):
			rekor.RunInvocationURI = strings.TrimPrefix(tag.Name, dependencytrack.RekorRunInvocationURITagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorSourceRepositoryOwnerURITagPrefix.String()):
			rekor.SourceRepositoryOwnerURI = strings.TrimPrefix(tag.Name, dependencytrack.RekorSourceRepositoryOwnerURITagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorBuildTriggerTagPrefix.String()):
			rekor.BuildTrigger = strings.TrimPrefix(tag.Name, dependencytrack.RekorBuildTriggerTagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorRunnerEnvironmentTagPrefix.String()):
			rekor.RunnerEnvironment = strings.TrimPrefix(tag.Name, dependencytrack.RekorRunnerEnvironmentTagPrefix.String())
		case strings.Contains(tag.Name, dependencytrack.RekorTagPrefix.String()):
			rekor.LogIndex = strings.TrimPrefix(tag.Name, dependencytrack.RekorTagPrefix.String())
		}
	}
	return rekor
}

func parseWorkloadRefTags(tags []dependencytrack.Tag) []*model.WorkloadReference {
	var workloads []*model.WorkloadReference
	for _, tag := range tags {
		if strings.Contains(tag.Name, dependencytrack.WorkloadTagPrefix.String()) {
			w := strings.TrimPrefix(tag.Name, dependencytrack.WorkloadTagPrefix.String())
			workload := strings.Split(w, "|")

			workloads = append(workloads, &model.WorkloadReference{
				ID:           scalar.WorkloadIdent(w),
				Environment:  workload[0],
				Team:         workload[1],
				WorkloadType: workload[2],
				Name:         workload[3],
			})
		}
	}
	return workloads
}

func (c *Client) GetFindingsForImageByProjectID(ctx context.Context, projectId string, suppressed bool) ([]*model.Finding, error) {
	findings, err := c.retrieveFindings(ctx, projectId, suppressed)
	if err != nil {
		return nil, fmt.Errorf("retrieving findings for project %s: %w", projectId, err)
	}

	retFindings := make([]*model.Finding, 0)
	for _, f := range findings {
		aliases := []*model.VulnIDAlias{}

		for _, alias := range f.Vulnerability.Aliases {
			if alias.CveId != "" {
				aliases = append(aliases, &model.VulnIDAlias{
					Name:   alias.CveId,
					Source: "NVD",
				})
			}
			if alias.GhsaId != "" {
				aliases = append(aliases, &model.VulnIDAlias{
					Name:   alias.GhsaId,
					Source: "GHSA",
				})
			}
		}

		retFindings = append(retFindings, &model.Finding{
			ID:              scalar.FindingIdent(f.Vulnerability.VulnId),
			ParentID:        projectId,
			VulnID:          f.Vulnerability.VulnId,
			VulnerabilityID: f.Vulnerability.UUID,
			Source:          f.Vulnerability.Source,
			ComponentID:     f.Component.UUID,
			Severity:        f.Vulnerability.Severity,
			Description:     f.Vulnerability.Title,
			PackageURL:      f.Component.PURL,
			Aliases:         aliases,
			IsSuppressed:    f.Analysis.IsSuppressed,
			State:           f.Analysis.State,
		})
	}
	return retFindings, nil
}

func (c *Client) GetMetadataForImageByProjectID(ctx context.Context, projectId string) (*model.ImageDetails, error) {
	p, err := c.retrieveProjectById(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("getting project by id %s: %w", projectId, err)
	}

	if p == nil {
		return nil, fmt.Errorf("project not found: %s", projectId)
	}

	return &model.ImageDetails{
		Name:      p.Name + ":" + p.Version,
		ID:        scalar.ImageIdent(p.Name, p.Version),
		Rekor:     parseRekorTags(p.Tags),
		Version:   p.Version,
		ProjectID: p.Uuid,
		HasSbom:   hasBom(p),
		Summary:   c.createSummaryForImage(p, hasBom(p)),
		GQLVars: model.ImageDetailsGQLVars{
			WorkloadReferences: parseWorkloadRefTags(p.Tags),
		},
	}, nil
}

func (c *Client) GetMetadataForTeam(ctx context.Context, team string) ([]*model.ImageDetails, error) {
	projects, err := c.retrieveProjectsForTeam(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("getting projects by team %s: %w", team, err)
	}

	if projects == nil {
		return nil, nil
	}

	images := make([]*model.ImageDetails, 0)
	for _, p := range projects {
		if p == nil {
			continue
		}

		// TODO: Find a better way to filter out these images
		if p.Name == "europe-north1-docker.pkg.dev/nais-io/nais/images/wonderwall" {
			continue
		}

		if p.Name == "europe-north1-docker.pkg.dev/nais-io/nais/images/elector" {
			continue
		}

		image := &model.ImageDetails{
			ID:        scalar.ImageIdent(p.Name, p.Version),
			ProjectID: p.Uuid,
			Name:      p.Name,
			Summary:   c.createSummaryForImage(p, hasBom(p)),
			Rekor:     parseRekorTags(p.Tags),
			Version:   p.Version,
			HasSbom:   hasBom(p),
			GQLVars: model.ImageDetailsGQLVars{
				WorkloadReferences: parseWorkloadRefTags(p.Tags),
			},
		}
		images = append(images, image)
	}

	return images, nil
}

func (c *Client) SuppressFinding(ctx context.Context, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy string, suppress bool) (*model.AnalysisTrail, error) {
	comment = fmt.Sprintf("on-behalf-of:%s|suppressed:%t|state:%s|comment:%s", suppressedBy, suppress, analysisState, comment)
	analysisRequest := &dependencytrack.AnalysisRequest{
		Vulnerability:         vulnerabilityID,
		Component:             componentID,
		Project:               projectID,
		AnalysisState:         analysisState,
		AnalysisJustification: "NOT_SET",
		AnalysisResponse:      "NOT_SET",
		Comment:               comment,
		IsSuppressed:          suppress,
	}

	err := c.client.RecordAnalysis(ctx, analysisRequest)
	if err != nil {
		return nil, fmt.Errorf("suppressing finding: %w", err)
	}

	trail, err := c.client.GetAnalysisTrail(ctx, projectID, componentID, vulnerabilityID)
	if err != nil {
		return nil, fmt.Errorf("getting analysis trail: %w", err)
	}

	if err = c.client.TriggerAnalysis(ctx, projectID); err != nil {
		return nil, fmt.Errorf("triggering analysis: %w", err)
	}

	return &model.AnalysisTrail{
		ID:           scalar.AnalysisTrailIdent(projectID, componentID, vulnerabilityID),
		State:        trail.AnalysisState,
		IsSuppressed: trail.IsSuppressed,
		GQLVars: model.AnalysisTrailGQLVars{
			Comments: parseComments(trail),
		},
	}, nil
}

func parseComments(trail *dependencytrack.Analysis) []*model.AnalysisComment {
	comments := make([]*model.AnalysisComment, 0)
	for _, comment := range trail.AnalysisComments {
		timestamp := time.Unix(int64(comment.Timestamp)/1000, 0).Local()
		after, found := strings.CutPrefix(comment.Comment, "on-behalf-of:")

		if found {
			onBehalfOf, theComment, _ := strings.Cut(after, "|")
			comment := &model.AnalysisComment{
				Comment:    theComment,
				Timestamp:  timestamp,
				OnBehalfOf: onBehalfOf,
			}
			comments = append(comments, comment)
		}
	}

	// sort comments on timestamp desc
	slices.SortFunc(comments, func(i, j *model.AnalysisComment) int {
		return int(j.Timestamp.Sub(i.Timestamp).Seconds())
	})

	return comments
}

func (c *Client) GetAnalysisTrailForImage(ctx context.Context, projectID, componentID, vulnerabilityID string) (*model.AnalysisTrail, error) {
	trail, err := c.client.GetAnalysisTrail(ctx, projectID, componentID, vulnerabilityID)
	if err != nil {
		return nil, fmt.Errorf("getting analysis trail: %w", err)
	}

	if trail == nil {
		return &model.AnalysisTrail{ID: scalar.AnalysisTrailIdent(projectID, componentID, vulnerabilityID)}, nil
	}

	retAnalysisTrail := &model.AnalysisTrail{
		ID:           scalar.AnalysisTrailIdent(projectID, componentID, vulnerabilityID),
		State:        trail.AnalysisState,
		IsSuppressed: trail.IsSuppressed,
		GQLVars: model.AnalysisTrailGQLVars{
			Comments: parseComments(trail),
		},
	}

	return retAnalysisTrail, nil
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
