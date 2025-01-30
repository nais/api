package deployment

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/deployment/deploymentsql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/role"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"k8s.io/utils/ptr"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*DeploymentConnection, error) {
	q := db(ctx)

	ret, err := q.ListByTeamSlug(ctx, deploymentsql.ListByTeamSlugParams{
		TeamSlug: teamSlug,
		Offset:   page.Offset(),
		Limit:    page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountForTeam(ctx, teamSlug)
	if err != nil {
		return nil, err
	}

	return pagination.NewConvertConnection(ret, page, total, toGraphDeployment), nil
}

func ListResourcesForDeployment(ctx context.Context, deploymentID uuid.UUID, page *pagination.Pagination) (*DeploymentResourceConnection, error) {
	q := db(ctx)

	ret, err := q.ListResourcesForDeployment(ctx, deploymentsql.ListResourcesForDeploymentParams{
		DeploymentID: deploymentID,
		Offset:       page.Offset(),
		Limit:        page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountResourcesForDeployment(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return pagination.NewConvertConnection(ret, page, total, toGraphDeploymentResource), nil
}

func ListStatusesForDeployment(ctx context.Context, deploymentID uuid.UUID, page *pagination.Pagination) (*DeploymentStatusConnection, error) {
	q := db(ctx)

	ret, err := q.ListStatusesForDeployment(ctx, deploymentsql.ListStatusesForDeploymentParams{
		DeploymentID: deploymentID,
		Offset:       page.Offset(),
		Limit:        page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := fromContext(ctx).deploymentStatusForDeploymentCountLoader.Load(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	return pagination.NewConvertConnection(ret, page, total.Count, toGraphDeploymentStatus), nil
}

func ListForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string, workloadType workload.Type, page *pagination.Pagination) (*DeploymentConnection, error) {
	q := db(ctx)

	ret, err := q.ListForWorkload(ctx, deploymentsql.ListForWorkloadParams{
		TeamSlug:        teamSlug,
		EnvironmentName: environmentName,
		WorkloadName:    workloadName,
		WorkloadKind:    workloadType.String(),
		Offset:          page.Offset(),
		Limit:           page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountForWorkload(ctx, deploymentsql.CountForWorkloadParams{
		TeamSlug:        teamSlug,
		EnvironmentName: environmentName,
		WorkloadName:    workloadName,
		WorkloadKind:    workloadType.String(),
	})
	if err != nil {
		return nil, err
	}

	return pagination.NewConvertConnection(ret, page, total, toGraphDeployment), nil
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

	err = activitylog.Create(ctx, activitylog.CreateInput{
		Action:       activitylog.ActivityLogEntryActionUpdated,
		Actor:        authz.ActorFromContext(ctx).User,
		ResourceType: activityLogEntryResourceTypeDeployKey,
		ResourceName: "deploy-key",
		TeamSlug:     ptr.To(teamSlug),
	})
	if err != nil {
		return nil, err
	}

	return toGraphDeploymentKey(dk, teamSlug), nil
}

func getDeploymentKeyByIdent(ctx context.Context, id ident.Ident) (*DeploymentKey, error) {
	teamSlug, err := parseDeploymentKeyIdent(id)
	if err != nil {
		return nil, err
	}
	// We ensure that the authenticated user has access to the deployment key first
	if err := authz.RequireTeamAuthorizationCtx(ctx, role.AuthorizationDeployKeyRead, teamSlug); err != nil {
		return nil, err
	}
	return KeyForTeam(ctx, teamSlug)
}

func get(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	deployment, err := fromContext(ctx).deploymentLoader.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return deployment, nil
}

func getDeploymentResource(ctx context.Context, id uuid.UUID) (*DeploymentResource, error) {
	resource, err := fromContext(ctx).deploymentResourceLoader.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return resource, nil
}

func getDeploymentStatus(ctx context.Context, id uuid.UUID) (*DeploymentStatus, error) {
	status, err := fromContext(ctx).deploymentStatusLoader.Load(ctx, id)
	if err != nil {
		return nil, err
	}
	return status, nil
}

func getDeploymentByIdent(ctx context.Context, id ident.Ident) (*Deployment, error) {
	uid, err := parseDeploymentIdent(id)
	if err != nil {
		return nil, err
	}
	return get(ctx, uid)
}

func getDeploymentResourceByIdent(ctx context.Context, id ident.Ident) (*DeploymentResource, error) {
	uid, err := parseDeploymentResourceIdent(id)
	if err != nil {
		return nil, err
	}
	return getDeploymentResource(ctx, uid)
}

func getDeploymentStatusByIdent(ctx context.Context, id ident.Ident) (*DeploymentStatus, error) {
	uid, err := parseDeploymentStatusIdent(id)
	if err != nil {
		return nil, err
	}
	return getDeploymentStatus(ctx, uid)
}

func latestDeploymentTimestampForWorkload(ctx context.Context, wl workload.Workload) (time.Time, error) {
	t, err := db(ctx).LatestDeploymentTimestampForWorkload(ctx, deploymentsql.LatestDeploymentTimestampForWorkloadParams{
		TeamSlug:        wl.GetTeamSlug(),
		EnvironmentName: wl.GetEnvironmentName(),
		WorkloadName:    wl.GetName(),
		WorkloadKind:    wl.GetType().String(),
	})
	if err != nil {
		return time.Time{}, err
	}

	return t.Time, nil
}
