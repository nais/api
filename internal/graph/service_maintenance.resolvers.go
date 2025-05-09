package graph

import (
	"context"

	"github.com/nais/api/internal/persistence/valkey"
	servicemaintenance "github.com/nais/api/internal/service_maintenance"
)

func (r *mutationResolver) RunMaintenance(ctx context.Context, input servicemaintenance.RunMaintenanceInput) (*string, error) {
	err := servicemaintenance.RunServiceMaintenance(ctx, input)
	if err != nil {
		return nil, err
	}
	string := "success"
	return &string, nil
}

func (r *valkeyInstanceResolver) Maintenance(ctx context.Context, obj *valkey.ValkeyInstance) (*servicemaintenance.ServiceMaintenance, error) {
	return servicemaintenance.GetServiceMaintenances(ctx, *obj)
}

func (r *valkeyInstanceResolver) Project(ctx context.Context, obj *valkey.ValkeyInstance) (string, error) {
	// TODO: Figure out if there's a better/more preferred way of doing this
	return obj.AivenProject, nil
}
