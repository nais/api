package deployment

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/workload"
	"k8s.io/utils/ptr"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*DeploymentConnection, error) {
	cluster, err := withCluster(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	all, err := fromContext(ctx).client.Deployments(ctx, hookd.WithTeam(teamSlug.String()), hookd.WithLimit(100), hookd.WithCluster(cluster))
	if err != nil {
		return nil, fmt.Errorf("getting deploys from Hookd: %w", err)
	}

	ret := pagination.Slice(all, page)
	return pagination.NewConvertConnection(ret, page, len(all), toGraphDeployment), nil
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, workloadType workload.Type, page *pagination.Pagination) (*DeploymentConnection, error) {
	cluster, err := withCluster(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	all, err := fromContext(ctx).client.Deployments(ctx, hookd.WithTeam(teamSlug.String()), hookd.WithCluster(environmentName), hookd.WithCluster(cluster))
	if err != nil {
		return nil, fmt.Errorf("getting deploys from Hookd: %w", err)
	}

	var kind string
	switch workloadType {
	case workload.TypeApplication:
		kind = "Application"
	case workload.TypeJob:
		kind = "Naisjob"
	default:
		return nil, fmt.Errorf("unsupported workload type: %v", workloadType)
	}

	filtered := make([]hookd.Deploy, 0)
deploys:
	for _, deploy := range all {
		for _, resource := range deploy.Resources {
			if resource.Name == workloadName && resource.Kind == kind {
				filtered = append(filtered, deploy)
				continue deploys
			}
		}
	}

	ret := pagination.Slice(filtered, page)
	return pagination.NewConvertConnection(ret, page, len(filtered), toGraphDeployment), nil
}

func KeyForTeam(ctx context.Context, teamSlug slug.Slug) (*DeploymentKey, error) {
	dk, err := fromContext(ctx).client.DeployKey(ctx, teamSlug.String())
	if err != nil {
		return nil, err
	}

	return toGraphDeploymentKey(dk, teamSlug), nil
}

func ChangeDeploymentKey(ctx context.Context, teamSlug slug.Slug) (*DeploymentKey, error) {
	dk, err := fromContext(ctx).client.ChangeDeployKey(ctx, teamSlug.String())
	if err != nil {
		return nil, err
	}

	err = audit.Create(ctx, audit.CreateInput{
		Action:       audit.AuditActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: auditResourceTypeDeployKey,
		ResourceName: "deploy-key",
		TeamSlug:     ptr.To(teamSlug),
	})
	if err != nil {
		return nil, err
	}

	return toGraphDeploymentKey(dk, teamSlug), nil
}

func InfoForWorkload(ctx context.Context, workload workload.Workload) (*DeploymentInfo, error) {
	valPtr := func(m map[string]string, key string) *string {
		if m == nil {
			return nil
		}

		if v, ok := m[key]; ok {
			return &v
		}
		return nil
	}

	an := workload.GetAnnotations()

	var timestamp *time.Time
	if ts := workload.GetRolloutCompleteTime(); ts > 0 {
		t := time.Unix(0, ts)
		timestamp = &t
	}

	return &DeploymentInfo{
		Deployer:        valPtr(an, "deploy.nais.io/github-actor"),
		CommitSha:       valPtr(an, "deploy.nais.io/github-sha"),
		URL:             valPtr(an, "deploy.nais.io/github-workflow-run-url"),
		Timestamp:       timestamp,
		TeamSlug:        workload.GetTeamSlug(),
		EnvironmentName: workload.GetEnvironmentName(),
		WorkloadName:    workload.GetName(),
		WorkloadType:    workload.GetType(),
	}, nil
}

func getDeploymentKeyByIdent(ctx context.Context, id ident.Ident) (*DeploymentKey, error) {
	// We ensure that the authenticated user has access to the deployment key first

	teamSlug, err := parseDeploymentKeyIdent(id)
	if err != nil {
		return nil, err
	}
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationDeployKeyRead, teamSlug); err != nil {
		return nil, err
	}
	return KeyForTeam(ctx, teamSlug)
}

func getDeploymentByIdent(ctx context.Context, id ident.Ident) (*Deployment, error) {
	return nil, apierror.Errorf("deployments are not accessible by node ID")
}

func withCluster(ctx context.Context, teamSlug slug.Slug) (string, error) {
	envs, err := team.ListTeamEnvironments(ctx, teamSlug)
	if err != nil {
		return "", err
	}

	names := make([]string, 0, len(envs))
	for _, env := range envs {
		names = append(names, env.Name)
	}

	return strings.Join(names, ","), nil
}
