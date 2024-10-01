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

var _ Client = &dependencyTrackClient{}

type Client interface {
	GetMetadataForImageByProjectID(ctx context.Context, projectID string) (*model.ImageDetails, error)
	GetMetadataForImage(ctx context.Context, image string) (*model.ImageDetails, error)
	GetFindingsForImageByProjectID(ctx context.Context, projectID string, suppressed bool) ([]*model.Finding, error)
	GetMetadataForTeam(ctx context.Context, team string) ([]*model.ImageDetails, error)
	GetVulnerabilityError(ctx context.Context, image string, revision string) (model.StateError, error)
	SuppressFinding(ctx context.Context, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy string, suppress bool) (*model.AnalysisTrail, error)
	GetAnalysisTrailForImage(ctx context.Context, projectID, componentID, vulnerabilityID string) (*model.AnalysisTrail, error)
	UploadProject(ctx context.Context, image, name, version, team string, bom []byte) error
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
	ch := cache.New(10*time.Minute, 5*time.Minute)

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
	name, version, _ := strings.Cut(image, ":")
	// TODO: vurder cache?
	p, err := c.client.GetProject(ctx, name, version)
	if err != nil {
		return nil, fmt.Errorf("getting project by name %s and version %s: %w", name, version, err)
	}

	sum := c.createSummaryForImage(p)

	switch getVulnerabilityState(sum) {
	case model.VulnerabilityStateOk:
		return nil, nil
	case model.VulnerabilityStateMissingSbom:
		return model.MissingSbomError{
			Revision: revision,
			Level:    model.ErrorLevelWarning,
		}, nil
	case model.VulnerabilityStateVulnerable:
		return model.VulnerableError{
			Revision: revision,
			Level:    model.ErrorLevelWarning,
			Summary:  sum,
		}, nil

	}
	return nil, nil
}

func (c *dependencyTrackClient) GetProjectMetrics(ctx context.Context, instance *WorkloadInstance, date string) (*ProjectMetric, error) {
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

func (c *dependencyTrackClient) GetMetadataForImageByProjectID(ctx context.Context, projectId string) (*model.ImageDetails, error) {
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
		HasSbom:   hasSbom(p),
		Summary:   c.createSummaryForImage(p),
		GQLVars: model.ImageDetailsGQLVars{
			WorkloadReferences: parseWorkloadRefTags(p.Tags),
		},
	}, nil
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

func (c *dependencyTrackClient) UploadProject(ctx context.Context, image, name, version, team string, bom []byte) error {
	tags := []string{
		dependencytrack.EnvironmentTagPrefix.With("dev"),
		dependencytrack.TeamTagPrefix.With(team),
		dependencytrack.WorkloadTagPrefix.With("dev|" + team + "|app|" + name),
		dependencytrack.ImageTagPrefix.With(image),
	}
	createdP, err := c.client.CreateProject(ctx, image, version, team, tags)
	if err != nil {
		// since we are creating the project, we can ignore the conflict error
		if !dependencytrack.IsConflict(err) {
			return fmt.Errorf("creating project: %w", err)
		}
		return nil
	}

	err = c.client.UploadProject(ctx, image, version, createdP.Uuid, false, bom)
	if err != nil {
		return fmt.Errorf("uploading bom: %w", err)
	}
	return nil
}

// Due to the nature of the DependencyTrack API, the 'LastBomImportFormat' is not reliable to determine if a project has a BOM.
// The 'LastBomImportFormat' can be empty even if the project has a BOM.
// As a fallback, we can check if projects has registered any components, then we assume that if a project has components, it has a BOM.
func hasSbom(p *dependencytrack.Project) bool {
	if p == nil {
		return false
	}
	return p.LastBomImportFormat != "" || p.Metrics != nil && p.Metrics.Components > 0
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

func (c *dependencyTrackClient) retrieveProjectById(ctx context.Context, projectId string) (*dependencytrack.Project, error) {
	project, err := c.client.GetProjectById(ctx, projectId)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	return project, nil
}

func (c *dependencyTrackClient) retrieveProjectsForTeam(ctx context.Context, team string) ([]*dependencytrack.Project, error) {
	tag := url.QueryEscape("team:" + team)

	projects, err := c.client.GetProjectsByTag(ctx, tag)
	if err != nil {
		return nil, fmt.Errorf("getting projects from DependencyTrack: %w", err)
	}

	return projects, nil
}

func (c *dependencyTrackClient) retrieveProject(ctx context.Context, instance *WorkloadInstance) (*dependencytrack.Project, error) {
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
