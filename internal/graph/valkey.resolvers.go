package graph

import (
	"context"
	"fmt"

	"github.com/davecgh/go-spew/spew"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
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
	spew.Dump(input)

	return nil, fmt.Errorf("not implemented: CreateValkey - create a Valkey instance is not yet implemented")
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

func (r *valkeyAccessResolver) Workload(ctx context.Context, obj *valkey.ValkeyAccess) (workload.Workload, error) {
	return getWorkload(ctx, obj.WorkloadReference, obj.TeamSlug, obj.EnvironmentName)
}

func (r *Resolver) Valkey() gengql.ValkeyResolver { return &valkeyResolver{r} }

func (r *Resolver) ValkeyAccess() gengql.ValkeyAccessResolver { return &valkeyAccessResolver{r} }

type (
	valkeyResolver       struct{ *Resolver }
	valkeyAccessResolver struct{ *Resolver }
)

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
/*
	func (r *createValkeyInputResolver) EnvironmentName(ctx context.Context, obj *valkey.CreateValkeyInput, data string) error {
	panic(fmt.Errorf("not implemented: EnvironmentName - environmentName"))
}
func (r *createValkeyInputResolver) TeamSlug(ctx context.Context, obj *valkey.CreateValkeyInput, data slug.Slug) error {
	panic(fmt.Errorf("not implemented: TeamSlug - teamSlug"))
}
func (r *Resolver) CreateValkeyInput() gengql.CreateValkeyInputResolver {
	return &createValkeyInputResolver{r}
}
*/
