package graph

import (
	"context"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
)

func resolveEventTeam(ctx context.Context, obj audit.BaseAuditEvent) (*model.Team, error) {
	if obj.GQLVars.Team == "" {
		return nil, nil
	}

	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func resolveEventEnv(ctx context.Context, obj audit.BaseAuditEvent) (*model.Env, error) {
	if obj.GQLVars.Environment == "" || obj.GQLVars.Team == "" {
		return nil, nil
	}

	return loader.GetTeamEnvironment(ctx, obj.GQLVars.Team, obj.GQLVars.Environment)
}
