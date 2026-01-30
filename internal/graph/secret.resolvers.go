package graph

import (
	"context"
	"slices"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/user"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	"github.com/nais/api/internal/workload/secret"
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
	if err := authz.CanCreateSecrets(ctx, input.Team); err != nil {
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
	if err := authz.CanUpdateSecrets(ctx, input.Team); err != nil {
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
	if err := authz.CanUpdateSecrets(ctx, input.Team); err != nil {
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
	if err := authz.CanUpdateSecrets(ctx, input.Team); err != nil {
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
	if err := authz.CanDeleteSecrets(ctx, input.Team); err != nil {
		return nil, err
	}

	if err := secret.Delete(ctx, input.Team, input.Environment, input.Name); err != nil {
		return nil, err
	}

	return &secret.DeleteSecretPayload{
		SecretDeleted: true,
	}, nil
}

func (r *mutationResolver) ViewSecretValues(ctx context.Context, input secret.ViewSecretValuesInput) (*secret.ViewSecretValuesPayload, error) {
	return secret.ViewSecretValues(ctx, input)
}

func (r *secretResolver) Environment(ctx context.Context, obj *secret.Secret) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *secretResolver) TeamEnvironment(ctx context.Context, obj *secret.Secret) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *secretResolver) Team(ctx context.Context, obj *secret.Secret) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *secretResolver) Applications(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*application.Application], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allApps := application.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)

	ret := make([]*application.Application, 0)
	for _, app := range allApps {
		if slices.Contains(app.GetSecrets(), obj.Name) {
			ret = append(ret, app)
		}
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, len(ret)), nil
}

func (r *secretResolver) Jobs(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*job.Job], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allJobs := job.ListAllForTeam(ctx, obj.TeamSlug, nil, nil)

	ret := make([]*job.Job, 0)
	for _, j := range allJobs {
		if slices.Contains(j.GetSecrets(), obj.Name) {
			ret = append(ret, j)
		}
	}

	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, len(ret)), nil
}

func (r *secretResolver) Workloads(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := make([]workload.Workload, 0)

	applications := application.ListAllForTeamInEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
	for _, app := range applications {
		if slices.Contains(app.GetSecrets(), obj.Name) {
			ret = append(ret, app)
		}
	}

	jobs := job.ListAllForTeamInEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
	for _, j := range jobs {
		if slices.Contains(j.GetSecrets(), obj.Name) {
			ret = append(ret, j)
		}
	}

	workloads := pagination.Slice(ret, page)
	slices.SortStableFunc(workloads, func(a, b workload.Workload) int {
		return model.Compare(a.GetName(), b.GetName(), model.OrderDirectionAsc)
	})
	return pagination.NewConnection(workloads, page, len(ret)), nil
}

func (r *secretResolver) LastModifiedBy(ctx context.Context, obj *secret.Secret) (*user.User, error) {
	if obj.ModifiedByUserEmail == nil {
		return nil, nil
	}

	return user.GetByEmail(ctx, *obj.ModifiedByUserEmail)
}

func (r *secretResolver) ActivityLog(ctx context.Context, obj *secret.Secret, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForResourceTeamAndEnvironment(
		ctx,
		"SECRET",
		obj.TeamSlug,
		obj.Name,
		environmentmapper.EnvironmentName(obj.EnvironmentName),
		page,
		filter,
	)
}

// Secrets returns all secrets for a team.
// Secret metadata (names, keys) is visible to all authenticated users.
// Secret values require team membership (use viewSecretValues mutation).
func (r *teamResolver) Secrets(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *secret.SecretOrder, filter *secret.SecretFilter) (*pagination.Connection[*secret.Secret], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return secret.ListForTeam(ctx, obj.Slug, page, orderBy, filter)
}

// Secret returns a single secret by name.
// Secret metadata (name, keys) is visible to all authenticated users.
// Secret values require team membership (use viewSecretValues mutation).
func (r *teamEnvironmentResolver) Secret(ctx context.Context, obj *team.TeamEnvironment, name string) (*secret.Secret, error) {
	return secret.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *Resolver) Secret() gengql.SecretResolver { return &secretResolver{r} }

type secretResolver struct{ *Resolver }
