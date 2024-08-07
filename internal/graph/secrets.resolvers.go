package graph

import (
	"context"
	"errors"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/slug"
)

func (r *mutationResolver) CreateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.VariableInput) (*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	ret, err := r.k8sClient.CreateSecret(ctx, name, team, env, data)
	if errors.Is(err, k8s.ErrSecretUnmanaged) {
		return nil, apierror.ErrSecretUnmanaged
	}
	if err != nil {
		return nil, err
	}

	err = r.auditor.SecretCreated(ctx, actor.User, team, name)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) UpdateSecret(ctx context.Context, name string, team slug.Slug, env string, data []*model.VariableInput) (*model.Secret, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return nil, err
	}

	ret, err := r.k8sClient.UpdateSecret(ctx, name, team, env, data)
	if errors.Is(err, k8s.ErrSecretUnmanaged) {
		return nil, apierror.ErrSecretUnmanaged
	}
	if err != nil {
		return nil, err
	}

	// TODO: split mutation (e.g. AddKeyValue, UpdateKeyValue, DeleteKeyValue) to allow more granular auditing?
	err = r.auditor.SecretUpdated(ctx, actor.User, team, name)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *mutationResolver) DeleteSecret(ctx context.Context, name string, team slug.Slug, env string) (bool, error) {
	actor := authz.ActorFromContext(ctx)
	err := authz.RequireTeamMembership(actor, team)
	if err != nil {
		return false, err
	}
	deleted, err := r.k8sClient.DeleteSecret(ctx, name, team, env)
	if errors.Is(err, k8s.ErrSecretUnmanaged) {
		return false, apierror.ErrSecretUnmanaged
	}
	if err != nil {
		return deleted, err
	}

	err = r.auditor.SecretDeleted(ctx, actor.User, team, name)
	if err != nil {
		return deleted, err
	}

	return deleted, nil
}

func (r *secretResolver) Env(ctx context.Context, obj *model.Secret) (*model.Env, error) {
	return &model.Env{Name: obj.GQLVars.Env, Team: obj.GQLVars.Team.String()}, nil
}

func (r *secretResolver) Team(ctx context.Context, obj *model.Secret) (*model.Team, error) {
	return loader.GetTeam(ctx, obj.GQLVars.Team)
}

func (r *secretResolver) Data(ctx context.Context, obj *model.Secret) ([]*model.Variable, error) {
	return convertSecretDataToVariables(obj.Data), nil
}

func (r *secretResolver) Apps(ctx context.Context, obj *model.Secret) ([]*model.App, error) {
	return r.k8sClient.AppsUsingSecret(ctx, obj)
}

func (r *secretResolver) Jobs(ctx context.Context, obj *model.Secret) ([]*model.NaisJob, error) {
	return r.k8sClient.NaisJobsUsingSecret(ctx, obj)
}

func (r *secretResolver) LastModifiedBy(ctx context.Context, obj *model.Secret) (*model.User, error) {
	if obj.GQLVars.LastModifiedBy == "" {
		return nil, nil
	}

	return r.Query().User(ctx, nil, &obj.GQLVars.LastModifiedBy)
}

func (r *Resolver) Secret() gengql.SecretResolver { return &secretResolver{r} }

type secretResolver struct{ *Resolver }
