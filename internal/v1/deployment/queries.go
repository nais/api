package deployment

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
)

func ForTeam(ctx context.Context, teamSlug slug.Slug) (*DeploymentKey, error) {
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
	return ForTeam(ctx, teamSlug)
}
