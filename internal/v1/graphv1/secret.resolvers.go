package graphv1

import (
	"context"
	"slices"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/user"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
	"github.com/nais/api/internal/v1/workload/secret"
)

func (r *applicationResolver) Secrets(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*secret.Secret], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return secret.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj, page)
}

func (r *jobResolver) Secrets(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*secret.Secret], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return secret.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj, page)
}

func (r *mutationResolver) CreateSecret(ctx context.Context, input secret.CreateSecretInput) (*secret.CreateSecretPayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.Team); err != nil {
		return nil, err
	}

	s, err := secret.Create(ctx, input.Team, input.Environment, input.Name)
	if err != nil {
		return nil, err
	}

	return &secret.CreateSecretPayload{
		Secret: s,
	}, nil
}

func (r *mutationResolver) AddSecretValue(ctx context.Context, input secret.AddSecretValueInput) (*secret.AddSecretValuePayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.Team); err != nil {
		return nil, err
	}

	s, err := secret.AddSecretValue(ctx, input.Team, input.Environment, input.Name, input.Value)
	if err != nil {
		return nil, err
	}

	return &secret.AddSecretValuePayload{
		Secret: s,
	}, nil
}

func (r *mutationResolver) UpdateSecretValue(ctx context.Context, input secret.UpdateSecretValueInput) (*secret.UpdateSecretValuePayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.Team); err != nil {
		return nil, err
	}

	s, err := secret.UpdateSecretValue(ctx, input.Team, input.Environment, input.Name, input.Value)
	if err != nil {
		return nil, err
	}

	return &secret.UpdateSecretValuePayload{
		Secret: s,
	}, nil
}

func (r *mutationResolver) RemoveSecretValue(ctx context.Context, input secret.RemoveSecretValueInput) (*secret.RemoveSecretValuePayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.Team); err != nil {
		return nil, err
	}

	s, err := secret.RemoveSecretValue(ctx, input.Team, input.Environment, input.SecretName, input.ValueName)
	if err != nil {
		return nil, err
	}

	return &secret.RemoveSecretValuePayload{
		Secret: s,
	}, nil
}

func (r *mutationResolver) DeleteSecret(ctx context.Context, input secret.DeleteSecretInput) (*secret.DeleteSecretPayload, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, input.Team); err != nil {
		return nil, err
	}

	if err := secret.Delete(ctx, input.Team, input.Environment, input.Name); err != nil {
		return nil, err
	}

	return &secret.DeleteSecretPayload{
		SecretDeleted: true,
	}, nil
}

func (r *secretResolver) Environment(ctx context.Context, obj *secret.Secret) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *secretResolver) Team(ctx context.Context, obj *secret.Secret) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *secretResolver) Values(ctx context.Context, obj *secret.Secret) ([]*secret.SecretValue, error) {
	if err := authz.RequireTeamMembershipCtx(ctx, obj.TeamSlug); err != nil {
		return nil, err
	}

	return secret.GetSecretValues(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *secretResolver) Applications(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*application.Application], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allApps := application.ListAllForTeam(ctx, obj.TeamSlug)

	ret := make([]*application.Application, 0)
	for _, app := range allApps {
		if slices.Contains(app.GetSecrets(), obj.Name) {
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
	for _, j := range allJobs {
		if slices.Contains(j.GetSecrets(), obj.Name) {
			ret = append(ret, j)
		}
	}

	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, int32(len(ret))), nil
}

func (r *secretResolver) Workloads(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := make([]workload.Workload, 0)

	applications := application.ListAllForTeam(ctx, obj.TeamSlug)
	for _, app := range applications {
		if slices.Contains(app.GetSecrets(), obj.Name) {
			ret = append(ret, app)
		}
	}

	jobs := job.ListAllForTeam(ctx, obj.TeamSlug)
	for _, j := range jobs {
		if slices.Contains(j.GetSecrets(), obj.Name) {
			ret = append(ret, j)
		}
	}

	workloads := pagination.Slice(ret, page)
	slices.SortStableFunc(workloads, func(a, b workload.Workload) int {
		return modelv1.Compare(a.GetName(), b.GetName(), modelv1.OrderDirectionAsc)
	})
	return pagination.NewConnection(workloads, page, int32(len(ret))), nil
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
