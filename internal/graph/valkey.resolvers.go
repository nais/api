package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func (r *applicationResolver) Valkeys(ctx context.Context, obj *application.Application, orderBy *valkey.ValkeyOrder) (*pagination.Connection[*valkey.Valkey], error) {
	return valkey.ListForWorkload(ctx, obj.TeamSlug, obj.GetEnvironmentName(), obj.Spec.Valkey, orderBy)
}

func (r *jobResolver) Valkeys(ctx context.Context, obj *job.Job, orderBy *valkey.ValkeyOrder) (*pagination.Connection[*valkey.Valkey], error) {
	return valkey.ListForWorkload(ctx, obj.TeamSlug, obj.GetEnvironmentName(), obj.Spec.Valkey, orderBy)
}

func (r *mutationResolver) CreateValkey(ctx context.Context, input valkey.CreateValkeyInput) (*valkey.CreateValkeyPayload, error) {
	if err := authz.CanCreateValkey(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return valkey.Create(ctx, input)
}

func (r *mutationResolver) UpdateValkey(ctx context.Context, input valkey.UpdateValkeyInput) (*valkey.UpdateValkeyPayload, error) {
	if err := authz.CanUpdateValkey(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return valkey.Update(ctx, input)
}

func (r *mutationResolver) DeleteValkey(ctx context.Context, input valkey.DeleteValkeyInput) (*valkey.DeleteValkeyPayload, error) {
	if err := authz.CanDeleteValkey(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	return valkey.Delete(ctx, input)
}

func (r *teamResolver) Valkeys(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *valkey.ValkeyOrder) (*pagination.Connection[*valkey.Valkey], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return valkey.ListForTeam(ctx, obj.Slug, page, orderBy)
}

func (r *teamEnvironmentResolver) Valkey(ctx context.Context, obj *team.TeamEnvironment, name string) (*valkey.Valkey, error) {
	return valkey.Get(ctx, obj.TeamSlug, obj.EnvironmentName, name)
}

func (r *teamInventoryCountsResolver) Valkeys(ctx context.Context, obj *team.TeamInventoryCounts) (*valkey.TeamInventoryCountValkeys, error) {
	return &valkey.TeamInventoryCountValkeys{
		Total: len(valkey.ListAllForTeam(ctx, obj.TeamSlug)),
	}, nil
}

func (r *valkeyResolver) Team(ctx context.Context, obj *valkey.Valkey) (*team.Team, error) {
	return team.Get(ctx, obj.TeamSlug)
}

func (r *valkeyResolver) Environment(ctx context.Context, obj *valkey.Valkey) (*team.TeamEnvironment, error) {
	return r.TeamEnvironment(ctx, obj)
}

func (r *valkeyResolver) TeamEnvironment(ctx context.Context, obj *valkey.Valkey) (*team.TeamEnvironment, error) {
	return team.GetTeamEnvironment(ctx, obj.TeamSlug, obj.EnvironmentName)
}

func (r *valkeyResolver) Access(ctx context.Context, obj *valkey.Valkey, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *valkey.ValkeyAccessOrder) (*pagination.Connection[*valkey.ValkeyAccess], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return valkey.ListAccess(ctx, obj, page, orderBy)
}

func (r *valkeyResolver) Workload(ctx context.Context, obj *valkey.Valkey) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *valkeyResolver) State(ctx context.Context, obj *valkey.Valkey) (valkey.ValkeyState, error) {
	return valkey.State(ctx, obj)
}

func (r *valkeyResolver) Issues(ctx context.Context, obj *valkey.Valkey, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor, orderBy *issue.IssueOrder, filter *issue.IssueFilter) (*pagination.Connection[issue.Issue], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	t := issue.ResourceTypeValkey
	f := &issue.IssueFilter{
		ResourceName: &obj.Name,
		ResourceType: &t,
		Environments: []string{obj.EnvironmentName},
	}
	if filter != nil {
		f.Severity = filter.Severity
		f.IssueType = filter.IssueType
	}

	return issue.ListIssues(ctx, obj.TeamSlug, page, orderBy, f)
}

func (r *valkeyAccessResolver) Workload(ctx context.Context, obj *valkey.ValkeyAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) Valkey() gengql.ValkeyResolver { return &valkeyResolver{r} }

func (r *Resolver) ValkeyAccess() gengql.ValkeyAccessResolver { return &valkeyAccessResolver{r} }

type (
	valkeyResolver       struct{ *Resolver }
	valkeyAccessResolver struct{ *Resolver }
)
