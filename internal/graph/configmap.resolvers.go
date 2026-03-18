package graph

import (
	"context"
	"errors"
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
	"github.com/nais/api/internal/workload/configmap"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) Configs(ctx context.Context, obj *application.Application, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*configmap.Config], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return configmap.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj, page)
}

func (r *configResolver) TeamEnvironment(ctx context.Context, obj *configmap.Config) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *configResolver) Team(ctx context.Context, obj *configmap.Config) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *configResolver) Values(ctx context.Context, obj *configmap.Config) ([]*configmap.ConfigValue, error) {
	return configmap.GetConfigValues(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
}

func (r *configResolver) Applications(ctx context.Context, obj *configmap.Config, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*application.Application], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allApps := application.ListAllForTeamInEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)

	ret := make([]*application.Application, 0)
	for _, app := range allApps {
		if slices.Contains(app.GetConfigs(), obj.Name) {
			ret = append(ret, app)
		}
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, len(ret)), nil
}

func (r *configResolver) Jobs(ctx context.Context, obj *configmap.Config, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*job.Job], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	allJobs := job.ListAllForTeamInEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)

	ret := make([]*job.Job, 0)
	for _, j := range allJobs {
		if slices.Contains(j.GetConfigs(), obj.Name) {
			ret = append(ret, j)
		}
	}

	jobs := pagination.Slice(ret, page)
	return pagination.NewConnection(jobs, page, len(ret)), nil
}

func (r *configResolver) Workloads(ctx context.Context, obj *configmap.Config, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[workload.Workload], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	ret := make([]workload.Workload, 0)

	applications := application.ListAllForTeamInEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
	for _, app := range applications {
		if slices.Contains(app.GetConfigs(), obj.Name) {
			ret = append(ret, app)
		}
	}

	jobs := job.ListAllForTeamInEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
	for _, j := range jobs {
		if slices.Contains(j.GetConfigs(), obj.Name) {
			ret = append(ret, j)
		}
	}

	slices.SortStableFunc(ret, func(a, b workload.Workload) int {
		return model.Compare(a.GetName(), b.GetName(), model.OrderDirectionAsc)
	})
	workloads := pagination.Slice(ret, page)
	return pagination.NewConnection(workloads, page, len(ret)), nil
}

func (r *configResolver) LastModifiedBy(ctx context.Context, obj *configmap.Config) (*user.User, error) {
	if obj.ModifiedByUserEmail == nil {
		return nil, nil
	}

	u, err := user.GetByEmail(ctx, *obj.ModifiedByUserEmail)
	if err != nil {
		var notFound user.ErrNotFound
		if errors.As(err, &notFound) {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func (r *configResolver) ActivityLog(ctx context.Context, obj *configmap.Config, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, filter *activitylog.ActivityLogFilter) (*pagination.Connection[activitylog.ActivityLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return activitylog.ListForResourceTeamAndEnvironment(
		ctx,
		"CONFIG",
		obj.TeamSlug,
		obj.Name,
		environmentmapper.EnvironmentName(obj.EnvironmentName),
		page,
		filter,
	)
}

func (r *jobResolver) Configs(ctx context.Context, obj *job.Job, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*configmap.Config], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return configmap.ListForWorkload(ctx, obj.TeamSlug, obj.EnvironmentName, obj, page)
}

func (r *mutationResolver) CreateConfig(ctx context.Context, input configmap.CreateConfigInput) (*configmap.CreateConfigPayload, error) {
	if err := authz.CanCreateConfigs(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	c, err := configmap.Create(ctx, input.TeamSlug, input.EnvironmentName, input.Name)
	if err != nil {
		return nil, err
	}

	return &configmap.CreateConfigPayload{
		Config: c,
	}, nil
}

func (r *mutationResolver) AddConfigValue(ctx context.Context, input configmap.AddConfigValueInput) (*configmap.AddConfigValuePayload, error) {
	if err := authz.CanUpdateConfigs(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	c, err := configmap.AddConfigValue(ctx, input.TeamSlug, input.EnvironmentName, input.Name, input.Value)
	if err != nil {
		return nil, err
	}

	return &configmap.AddConfigValuePayload{
		Config: c,
	}, nil
}

func (r *mutationResolver) UpdateConfigValue(ctx context.Context, input configmap.UpdateConfigValueInput) (*configmap.UpdateConfigValuePayload, error) {
	if err := authz.CanUpdateConfigs(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	c, err := configmap.UpdateConfigValue(ctx, input.TeamSlug, input.EnvironmentName, input.Name, input.Value)
	if err != nil {
		return nil, err
	}

	return &configmap.UpdateConfigValuePayload{
		Config: c,
	}, nil
}

func (r *mutationResolver) RemoveConfigValue(ctx context.Context, input configmap.RemoveConfigValueInput) (*configmap.RemoveConfigValuePayload, error) {
	if err := authz.CanUpdateConfigs(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	c, err := configmap.RemoveConfigValue(ctx, input.TeamSlug, input.EnvironmentName, input.ConfigName, input.ValueName)
	if err != nil {
		return nil, err
	}

	return &configmap.RemoveConfigValuePayload{
		Config: c,
	}, nil
}

func (r *mutationResolver) DeleteConfig(ctx context.Context, input configmap.DeleteConfigInput) (*configmap.DeleteConfigPayload, error) {
	if err := authz.CanDeleteConfigs(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := configmap.Delete(ctx, input.TeamSlug, input.EnvironmentName, input.Name); err != nil {
		return nil, err
	}

	return &configmap.DeleteConfigPayload{
		ConfigDeleted: true,
	}, nil
}

// Configs returns all configs for a team.
func (r *teamResolver) Configs(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *configmap.ConfigOrder, filter *configmap.ConfigFilter) (*pagination.Connection[*configmap.Config], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return configmap.ListForTeam(ctx, obj.Slug, page, orderBy, filter)
}

// Config returns a single config by name.
func (r *teamEnvironmentResolver) Config(ctx context.Context, obj *team.TeamEnvironment, name string) (*configmap.Config, error) {
	return configmap.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) Configs(ctx context.Context, obj *team.TeamInventoryCounts) (*configmap.TeamInventoryCountConfigs, error) {
	return &configmap.TeamInventoryCountConfigs{
		Total: configmap.CountForTeam(ctx, obj.TeamSlug),
	}, nil
}

func (r *Resolver) Config() gengql.ConfigResolver { return &configResolver{r} }

type configResolver struct{ *Resolver }
