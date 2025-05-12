package graph

import (
	"context"

	"github.com/nais/api/internal/persistence/valkey"
	servicemaintenance "github.com/nais/api/internal/service_maintenance"
)

func (r *mutationResolver) RunMaintenance(ctx context.Context, input servicemaintenance.RunMaintenanceInput) (*servicemaintenance.RunMaintenancePayload, error) {
	err := servicemaintenance.RunServiceMaintenance(ctx, input)
	if err != nil {
		return nil, err
	}

	return &servicemaintenance.RunMaintenancePayload{
		Error: new(string),
	}, nil
}

func (r *valkeyInstanceResolver) Maintenance(ctx context.Context, obj *valkey.ValkeyInstance) (*servicemaintenance.ServiceMaintenance, error) {
	return servicemaintenance.GetServiceMaintenances(ctx, *obj)
}

func (r *valkeyInstanceResolver) Project(ctx context.Context, obj *valkey.ValkeyInstance) (string, error) {
	// TODO: Figure out if there's a better/more preferred way of doing this
	return obj.AivenProject, nil
}

// !!! WARNING !!!
// The code below was going to be deleted when updating resolvers. It has been copied here so you have
// one last chance to move it out of harms way if you want. There are two reasons this happens:
//  - When renaming or deleting a resolver the old code will be put in here. You can safely delete
//    it when you're done.
//  - You have helper methods in this file. Move them out to keep these resolver files clean.
/*
	func (r *runMaintenancePayloadResolver) Error(ctx context.Context, obj *servicemaintenance.RunMaintenancePayload) (*string, error) {
	if obj.Error == nil {
		return nil, nil
	}

	return obj.Error, nil
}
func (r *Resolver) RunMaintenancePayload() gengql.RunMaintenancePayloadResolver {
	return &runMaintenancePayloadResolver{r}
}
type runMaintenancePayloadResolver struct{ *Resolver }
*/
