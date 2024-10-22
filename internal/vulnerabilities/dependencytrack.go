package vulnerabilities

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	dependencytrack "github.com/nais/dependencytrack/pkg/client"
	"github.com/patrickmn/go-cache"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

const (
	dependencyTrackAdminTeam = "Administrators"
)

var imagesToExclude = []string{
	"europe-north1-docker.pkg.dev/nais-io/nais/images/wonderwall",
	"europe-north1-docker.pkg.dev/nais-io/nais/images/elector@",
}

var _ Client = &dependencyTrackClient{}

type Client interface {
	GetMetadataForImage(ctx context.Context, image string) (*model.ImageDetails, error)
	GetFindingsForImageByProjectID(ctx context.Context, projectID string, suppressed bool) ([]*model.Finding, error)
	GetMetadataForTeam(ctx context.Context, team string) ([]*model.ImageDetails, error)
	GetVulnerabilityError(ctx context.Context, image string, revision string) (model.StateError, error)
	SuppressFinding(ctx context.Context, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy string, suppress bool) (*model.AnalysisTrail, error)
	GetAnalysisTrailForImage(ctx context.Context, projectID, componentID, vulnerabilityID string) (*model.AnalysisTrail, error)
}

type dependencyTrackClient struct {
	client      dependencytrack.Client
	frontendUrl string
	log         logrus.FieldLogger
	cache       *cache.Cache
}

type DependencyTrackConfig struct {
	Endpoint, Username, Password, FrontendUrl string
	EnableFakes                               bool
}

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

type DependencyTrackOption func(*dependencyTrackClient)

func NewDependencyTrackClient(cfg DependencyTrackConfig, log *logrus.Entry, opts ...DependencyTrackOption) Client {
	c := dependencytrack.New(
		cfg.Endpoint,
		cfg.Username,
		cfg.Password,
		dependencytrack.WithApiKeySource(dependencyTrackAdminTeam),
		dependencytrack.WithLogger(log),
		dependencytrack.WithHttpClient(&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}),
	)
	ch := cache.New(2*time.Minute, 5*time.Minute)

	if cfg.EnableFakes {
		c = NewFakeDependencyTrackClient(c)
	}

	dc := &dependencyTrackClient{
		client:      c,
		frontendUrl: cfg.FrontendUrl,
		log:         log,
		cache:       ch,
	}

	for _, opt := range opts {
		opt(dc)
	}

	return dc
}

func WithClient(client dependencytrack.Client) DependencyTrackOption {
	return func(c *dependencyTrackClient) {
		c.client = client
	}
}

func (c *dependencyTrackClient) GetVulnerabilityError(ctx context.Context, image string, revision string) (model.StateError, error) {
	p, err := c.retrieveProject(ctx, image)
	if err != nil {
		return nil, fmt.Errorf("getting project by image %s: %w", image, err)
	}

	sum := c.createSummaryForImage(p)
	return stateToVulnerabilityError(sum, revision), nil
}

func (c *dependencyTrackClient) GetMetadataForImage(ctx context.Context, image string) (*model.ImageDetails, error) {
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
			Summary: c.createSummaryForImage(nil),
			Rekor:   model.Rekor{},
		}, nil
	}

	return &model.ImageDetails{
		Name:       p.Name + ":" + p.Version,
		ID:         scalar.ImageIdent(p.Name, p.Version),
		Rekor:      parseRekorTags(p.Tags),
		Version:    p.Version,
		HasSbom:    hasSbom(p),
		ProjectID:  p.Uuid,
		Summary:    c.createSummaryForImage(p),
		ProjectURL: c.frontendUrl + "/projects/" + p.Uuid,
		GQLVars: model.ImageDetailsGQLVars{
			WorkloadReferences: parseWorkloadRefTags(p.Tags),
		},
	}, nil
}

func (c *dependencyTrackClient) GetFindingsForImageByProjectID(ctx context.Context, projectId string, suppressed bool) ([]*model.Finding, error) {
	findings, err := c.retrieveFindings(ctx, projectId, suppressed)
	if err != nil {
		return nil, fmt.Errorf("retrieving findings for project %s: %w", projectId, err)
	}

	foundFindings := map[string]bool{}
	retFindings := make([]*model.Finding, 0)
	for _, f := range findings {
		// skip if we already have the finding
		if found := foundFindings[f.Vulnerability.VulnId]; found {
			continue
		}

		var aliases []*model.VulnIDAlias
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
		foundFindings[f.Vulnerability.VulnId] = true
	}
	return retFindings, nil
}

func (c *dependencyTrackClient) GetMetadataForTeam(ctx context.Context, team string) ([]*model.ImageDetails, error) {
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

		// skip platform images as the team does not own them
		if excludeProject(p) {
			continue
		}

		image := &model.ImageDetails{
			ID:        scalar.ImageIdent(p.Name, p.Version),
			ProjectID: p.Uuid,
			Name:      p.Name,
			Summary:   c.createSummaryForImage(p),
			Rekor:     parseRekorTags(p.Tags),
			Version:   p.Version,
			HasSbom:   hasSbom(p),
			GQLVars: model.ImageDetailsGQLVars{
				WorkloadReferences: parseWorkloadRefTags(p.Tags),
			},
		}
		images = append(images, image)
	}

	return images, nil
}

func (c *dependencyTrackClient) SuppressFinding(ctx context.Context, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy string, suppress bool) (*model.AnalysisTrail, error) {
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

func (c *dependencyTrackClient) GetAnalysisTrailForImage(ctx context.Context, projectID, componentID, vulnerabilityID string) (*model.AnalysisTrail, error) {
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

// Due to the nature of the DependencyTrack API, the 'LastBomImportFormat' is not reliable to determine if a project has a BOM.
// The 'LastBomImportFormat' can be empty even if the project has a BOM.
// As a fallback, we can check if projects has registered any components, then we assume that if a project has components, it has a BOM.
func hasSbom(p *dependencytrack.Project) bool {
	if p == nil {
		return false
	}

	return p.Metrics != nil && p.Metrics.Components > 0
}

func (c *dependencyTrackClient) retrieveFindings(ctx context.Context, uuid string, suppressed bool) ([]*dependencytrack.Finding, error) {
	if v, ok := c.cache.Get(uuid); ok {
		return v.([]*dependencytrack.Finding), nil
	}

	findings, err := c.client.GetFindings(ctx, uuid, suppressed)
	if err != nil {
		return nil, fmt.Errorf("retrieveFindings from DependencyTrack: %w", err)
	}

	c.cache.Set(uuid, findings, cache.DefaultExpiration)

	return findings, nil
}

func (c *dependencyTrackClient) createSummaryForImage(project *dependencytrack.Project) *model.ImageVulnerabilitySummary {
	if !hasSbom(project) {
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

func (c *dependencyTrackClient) retrieveProjectsForTeam(ctx context.Context, team string) ([]*dependencytrack.Project, error) {
	teamTag := dependencytrack.TeamTagPrefix.With(team)
	tag := url.QueryEscape(teamTag)
	if v, ok := c.cache.Get(teamTag); ok {
		c.log.Debugf("retrieved %d projects for team %s from cache", len(v.([]*dependencytrack.Project)), teamTag)
		return v.([]*dependencytrack.Project), nil
	}

	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	c.log.Debugf("retrieved %d projects for team %s", len(projects), teamTag)
	for _, project := range projects {
		if project == nil {
			continue
		}
		instanceImageTag := dependencytrack.ImageTagPrefix.With(project.Name + ":" + project.Version)
		c.cache.Set(instanceImageTag, project, cache.DefaultExpiration)
	}

	c.cache.Set(teamTag, projects, cache.DefaultExpiration)

	return projects, nil
}

func (c *dependencyTrackClient) retrieveProject(ctx context.Context, image string) (*dependencytrack.Project, error) {
	instanceImageTag := dependencytrack.ImageTagPrefix.With(image)
	tag := url.QueryEscape(instanceImageTag)

	if v, ok := c.cache.Get(instanceImageTag); ok {
		c.log.Debugf("retrieved project for image %s from cache", image)
		return v.(*dependencytrack.Project), nil
	}

	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	if len(projects) == 0 {
		return nil, nil
	}

	c.log.Debugf("retrieved %d projects for image %s", len(projects), image)
	var p *dependencytrack.Project
	for _, project := range projects {
		if containsAllTags(project.Tags, instanceImageTag) {
			p = project
			c.cache.Set(instanceImageTag, p, cache.DefaultExpiration)
			break
		}
	}
	return p, nil
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
		case strings.Contains(tag.Name, dependencytrack.DigestTagPrefix.String()):
			rekor.ImageDigestSha = strings.TrimPrefix(tag.Name, dependencytrack.DigestTagPrefix.String())
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

func parseComments(trail *dependencytrack.Analysis) []*model.AnalysisComment {
	comments := make([]*model.AnalysisComment, 0)
	if trail == nil {
		return comments
	}
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

func excludeProject(p *dependencytrack.Project) bool {
	for _, i := range imagesToExclude {
		if i == p.Name {
			return true
		}
	}
	return false
}
