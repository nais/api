package vulnerabilities

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	promapi "github.com/prometheus/client_golang/api"
	promv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	log "github.com/sirupsen/logrus"
)

type Manager struct {
	Client
	prometheusClients clusterPrometheusClients
	cfg               *Config
}

type Config struct {
	DependencyTrack DependencyTrackConfig
	Prometheus      PrometheusConfig
}

type (
	clusterPrometheusClients map[string]Prometheus
)

type Options = func(*Manager)

func NewManager(cfg *Config, opts ...Options) *Manager {
	m := &Manager{
		cfg: cfg,
	}

	for _, opt := range opts {
		opt(m)
	}

	if m.Client == nil {
		m.Client = NewDependencyTrackClient(
			cfg.DependencyTrack,
			log.WithField("client", "dependencytrack"),
		)
	}

	if m.prometheusClients == nil {
		prometheusClientMap, err := cfg.prometheusClients()
		if err != nil {
			log.WithError(err).Fatal("Failed to create prometheus clients")
		}
		m.prometheusClients = prometheusClientMap
	}

	return m
}

func WithDependencyTrackClient(client Client) func(*Manager) {
	return func(m *Manager) {
		m.Client = client
	}
}

func WithPrometheusClients(clients clusterPrometheusClients) func(*Manager) {
	return func(m *Manager) {
		m.prometheusClients = clients
	}
}

func (m *Manager) GetVulnerabilitiesForTeam(ctx context.Context, workloads []model.Workload, team string) ([]*model.VulnerabilityNode, error) {
	images, err := m.GetMetadataForTeam(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("getting metadata for team %q: %w", team, err)
	}

	nodes := make([]*model.VulnerabilityNode, 0)
	for _, workload := range workloads {
		env, wType, name := workloadDetails(workload)
		if env == "" || wType == "" || name == "" {
			continue
		}

		node := &model.VulnerabilityNode{
			ID:           scalar.VulnerabilitiesIdent(fmt.Sprintf("%s:%s:%s:%s", env, team, wType, name)),
			Env:          env,
			WorkloadType: wType,
			WorkloadName: name,
		}

		image := getImageDetails(images, env, team, wType, name)
		if image != nil {
			node.HasSbom = image.HasSbom
			node.Summary = image.Summary
		}

		nodes = append(nodes, node)
	}

	return nodes, nil
}

func (m *Manager) GetSummaryForTeam(ctx context.Context, workloads []model.Workload, team string, totalTeams int) (*model.VulnerabilitySummaryForTeam, error) {
	images, err := m.GetMetadataForTeam(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("getting metadata for team %q: %w", team, err)
	}

	vulnWorkloads := 0
	retVal := &model.VulnerabilitySummaryForTeam{}
	for _, workload := range workloads {
		env, wType, name := workloadDetails(workload)
		retVal.TotalWorkloads += 1

		image := getImageDetails(images, env, team, wType, name)
		if image == nil {
			continue
		}

		retVal.Critical += image.Summary.Critical
		retVal.High += image.Summary.High
		retVal.Medium += image.Summary.Medium
		retVal.Low += image.Summary.Low
		retVal.Unassigned += image.Summary.Unassigned
		retVal.RiskScore += image.Summary.RiskScore
		retVal.BomCount += 1

		s := getVulnerabilityState(image.Summary)
		if s == model.VulnerabilityStateVulnerable {
			vulnWorkloads += 1
		}
	}

	if len(workloads) == 0 {
		retVal.Coverage = 0.0
	} else {
		retVal.Coverage = float64(retVal.BomCount) / float64(retVal.TotalWorkloads) * 100
	}

	if retVal.Coverage < 90 {
		retVal.Status = append(retVal.Status, &model.VulnerabilityStatus{
			State:       model.VulnerabilityStateCoverageTooLow,
			Title:       "SBOM coverage",
			Description: "SBOM coverage is below 90% (number of workloads with SBOM / total number of workloads)",
		})
	}

	if vulnWorkloads > 0 {
		retVal.Status = append(retVal.Status, &model.VulnerabilityStatus{
			State:       model.VulnerabilityStateTooManyVulnerableWorkloads,
			Title:       "Too many vulnerable workloads",
			Description: "The threshold for a vulnerable workload is a riskscore above 100 or a critical vulnerability",
		})
	}

	ranking, err := m.teamRanking(ctx, team, totalTeams)
	if err != nil {
		return nil, fmt.Errorf("getting team ranking: %w", err)
	}
	retVal.TeamRanking = ranking

	return retVal, nil
}

func (m *Manager) GetVulnerabilityErrors(ctx context.Context, workloads []model.Workload, team string) ([]model.StateError, error) {
	images, err := m.GetMetadataForTeam(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("getting metadata for team %q: %w", team, err)
	}

	errors := make([]model.StateError, 0)
	for _, workload := range workloads {
		env, wType, name := workloadDetails(workload)
		if env == "" || wType == "" || name == "" {
			continue
		}

		var summary *model.ImageVulnerabilitySummary
		if image := getImageDetails(images, env, team, wType, name); image != nil {
			summary = image.Summary
		}

		revision := ""
		switch w := workload.(type) {
		case *model.App:
			revision = w.DeployInfo.CommitSha
		case *model.NaisJob:
			revision = w.DeployInfo.CommitSha
		}

		vulnErr := stateToVulnerabilityError(summary, revision)
		if vulnErr != nil {
			errors = append(errors, vulnErr)
		}
	}

	return errors, nil
}

func stateToVulnerabilityError(sum *model.ImageVulnerabilitySummary, revision string) model.StateError {
	switch getVulnerabilityState(sum) {
	case model.VulnerabilityStateOk:
		return nil
	case model.VulnerabilityStateMissingSbom:
		return model.MissingSbomError{
			Revision: revision,
			Level:    model.ErrorLevelWarning,
		}
	case model.VulnerabilityStateVulnerable:
		return model.VulnerableError{
			Revision: revision,
			Level:    model.ErrorLevelWarning,
			Summary:  sum,
		}
	}
	return nil
}

func getVulnerabilityState(summary *model.ImageVulnerabilitySummary) model.VulnerabilityState {
	switch {
	case summary == nil:
		return model.VulnerabilityStateMissingSbom
	case summary.Critical > 0:
		return model.VulnerabilityStateVulnerable
	// if the amount of high vulnerabilities is greater than 50 % of the total amount of vulnerabilities, we consider the image as vulnerable
	case summary.RiskScore > 100 && summary.High > summary.RiskScore/2:
		return model.VulnerabilityStateVulnerable
	}

	return model.VulnerabilityStateOk
}

func (m *Manager) teamRanking(ctx context.Context, team string, teams int) (model.VulnerabilityRanking, error) {
	currentRank, err := m.ranking(ctx, team, time.Now())
	if err != nil {
		return model.VulnerabilityRanking{}, fmt.Errorf("getting team ranking: %w", err)
	}

	previousRank, err := m.ranking(ctx, team, time.Now().AddDate(0, -1, 0))
	if err != nil {
		return model.VulnerabilityRanking{}, fmt.Errorf("getting previous ranking: %w", err)
	}

	retVal := model.VulnerabilityRanking{
		Rank:       currentRank,
		TotalTeams: teams,
	}

	switch {
	case currentRank > previousRank:
		retVal.Trend = model.VulnerabilityRankingTrendDown
	case currentRank < previousRank:
		retVal.Trend = model.VulnerabilityRankingTrendUp
	default:
		retVal.Trend = model.VulnerabilityRankingTrendFlat
	}

	// Divide teams into three parts
	upperLimit := teams / 3        // Upper third
	middleLimit := 2 * (teams / 3) // Middle third (everything before bottom third)

	// Determine vulnerable score based on rank
	switch {
	case currentRank <= upperLimit: // Top third
		retVal.VulnerableScore = model.VulnerableScoreUpper
	case currentRank > upperLimit && currentRank <= middleLimit: // Middle third
		retVal.VulnerableScore = model.VulnerableScoreMiddle
	default: // Bottom third
		retVal.VulnerableScore = model.VulnerableScoreBottom
	}

	return retVal, nil
}

func getImageDetails(images []*model.ImageDetails, env, team, wType, name string) (image *model.ImageDetails) {
	for _, i := range images {
		if i.Summary == nil {
			continue
		}
		if i.GQLVars.ContainsReference(env, team, wType, name) {
			return i
		}
	}
	return nil
}

func workloadDetails(workload model.Workload) (env, wType, name string) {
	switch w := workload.(type) {
	case *model.App:
		return w.Env.Name, "app", w.Name
	case *model.NaisJob:
		return w.Env.Name, "job", w.Name
	}
	return "", "", ""
}

func (c *Config) prometheusClients() (clusterPrometheusClients, error) {
	clients := clusterPrometheusClients{}
	for _, cluster := range c.Prometheus.Clusters {
		if c.Prometheus.EnableFakes {
			clients[cluster] = NewFakePrometheusClient()
			continue
		}
		prometheusUrl := fmt.Sprintf("https://nais-prometheus.%s.%s.cloud.nais.io", cluster, c.Prometheus.Tenant)
		promClient, err := promapi.NewClient(promapi.Config{
			Address: prometheusUrl,
		})
		if err != nil {
			return nil, err
		}
		clients[cluster] = promv1.NewAPI(promClient)
	}

	return clients, nil
}
