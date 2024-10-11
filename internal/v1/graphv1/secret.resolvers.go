package graphv1

import (
	"context"
	"slices"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/user"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/nais/api/internal/v1/workload/secret"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func (r *mutationResolver) CreateSecret(ctx context.Context, input secret.CreateSecretInput) (*secret.CreateSecretPayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.Team); err != nil {
		return nil, err
	}

	s, err := secret.Create(ctx, input.Team, input.Environment, input.Name, input.Data)
	if err != nil {
		return nil, err
	}

	return &secret.CreateSecretPayload{
		Secret: s,
	}, nil
}

func (r *secretResolver) Environment(ctx context.Context, obj *secret.Secret) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *secretResolver) Team(ctx context.Context, obj *secret.Secret) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *secretResolver) Data(ctx context.Context, obj *secret.Secret) ([]*secret.SecretVariable, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, obj.TeamSlug); err != nil {
		return nil, nil
	}

	return secret.GetSecretData(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *secretResolver) Applications(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*application.Application], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allApps := application.ListAllForTeam(ctx, obj.TeamSlug)

	ret := make([]*application.Application, 0)
	for _, app := range allApps {
		ok := slices.ContainsFunc(app.Spec.EnvFrom, func(o nais_io_v1.EnvFrom) bool {
			return o.Secret == obj.Name
		})
		if ok {
			ret = append(ret, app)
		}
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, int32(len(ret))), nil
}

func (r *secretResolver) Jobs(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*job.Job], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allJobs := job.ListAllForTeam(ctx, obj.TeamSlug)

	ret := make([]*job.Job, 0)
	for _, app := range allJobs {
		ok := slices.ContainsFunc(app.Spec.EnvFrom, func(o nais_io_v1.EnvFrom) bool {
			return o.Secret == obj.Name
		})
		if ok {
			ret = append(ret, app)
		}
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, int32(len(ret))), nil
}

func (r *secretResolver) LastModifiedBy(ctx context.Context, obj *secret.Secret) (*user.User, error) {
	if obj.ModifiedByUserEmail == nil {
		return nil, nil
	}

	return user.GetByEmail(ctx, *obj.ModifiedByUserEmail)
}

func (r *teamResolver) Secrets(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*secret.Secret], error) {
	if err := authz.RequireTeamMembershipCtx(ctx, obj.Slug); err != nil {
		return nil, nil
	}

	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return secret.ListForTeam(ctx, obj.Slug, page)
}

func (r *teamEnvironmentResolver) Secret(ctx context.Context, obj *team.TeamEnvironment, name string) (*secret.Secret, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, obj.TeamSlug); err != nil {
		return nil, nil
	}

	return secret.Get(ctx, obj.TeamSlug, obj.Name, name)
}

func (r *Resolver) Secret() gengqlv1.SecretResolver { return &secretResolver{r} }

type secretResolver struct{ *Resolver }
