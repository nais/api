package graph

import (
	"context"
	"errors"
	"fmt"

	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

func (r *envResolver) ID(ctx context.Context, obj *model.Env) (*scalar.Ident, error) {
	id := scalar.EnvIdent(obj.Name)
	return &id, nil
}

func (r *envResolver) GcpProjectID(ctx context.Context, obj *model.Env) (*string, error) {
	if obj.DBType == nil {
		te, err := loader.GetTeamEnvironment(ctx, slug.Slug(obj.Team), obj.Name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		obj = te
	}

	return obj.DBType.GcpProjectID, nil
}

func (r *envResolver) SlackAlertsChannel(ctx context.Context, obj *model.Env) (string, error) {
	if obj.DBType == nil {
		te, err := loader.GetTeamEnvironment(ctx, slug.Slug(obj.Team), obj.Name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				team, err := loader.GetTeam(ctx, slug.Slug(obj.Team))
				if err != nil {
					return "", fmt.Errorf("unable to load team: %w", err)
				}
				return team.SlackChannel, nil
			}
			return "", err
		}
		obj = te
	}

	return obj.DBType.SlackAlertsChannel, nil
}

func (r *envResolver) Secrets(ctx context.Context, obj *model.Env) ([]*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, slug.Slug(obj.Team))
	if err != nil {
		return nil, err
	}
	return r.k8sClient.SecretsForEnv(ctx, slug.Slug(obj.Team), obj.Name)
}

func (r *Resolver) Env() gengql.EnvResolver { return &envResolver{r} }

type envResolver struct{ *Resolver }
