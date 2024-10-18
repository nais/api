package deployment

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

func ListForTeam(ctx context.Context, teamSlug slug.Slug, page *pagination.Pagination) (*DeploymentConnection, error) {
	all, err := fromContext(ctx).client.Deployments(ctx, hookd.WithTeam(teamSlug.String()), hookd.WithLimit(100))
	if err != nil {
		return nil, fmt.Errorf("getting deploys from Hookd: %w", err)
	}

	ret := pagination.Slice(all, page)
	return pagination.NewConvertConnection(ret, page, int32(len(all)), toGraphDeployment), nil
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

	return toGraphDeploymentKey(dk, teamSlug), nil
}

func getByIdent(ctx context.Context, id ident.Ident) (*DeploymentKey, error) {
	// We ensure that the authenticated user has access to the deployment key first

	teamSlug, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	if err := authz.RequireTeamMembershipCtx(ctx, teamSlug); err != nil {
		return nil, err
	}
	return KeyForTeam(ctx, teamSlug)
}
