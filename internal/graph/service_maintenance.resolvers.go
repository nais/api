package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/persistence/valkey"
	servicemaintenance "github.com/nais/api/internal/service_maintenance"
)

func (r *mutationResolver) RunValkeyMaintenance(ctx context.Context, input servicemaintenance.RunValkeyMaintenanceInput) (*servicemaintenance.RunAivenMaintenancePayload, error) {
	if err := authz.CanStartServiceMaintenance(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	err := servicemaintenance.RunServiceMaintenance(ctx, input)
	if err != nil {
		return nil, err
	}

	return &servicemaintenance.RunAivenMaintenancePayload{
		Error: new(string),
	}, nil
}

func (r *valkeyInstanceResolver) Maintenance(ctx context.Context, obj *valkey.ValkeyInstance) (*servicemaintenance.ServiceMaintenance, error) {
	return servicemaintenance.GetServiceMaintenances(ctx, *obj)
}

func (r *valkeyInstanceResolver) Project(ctx context.Context, obj *valkey.ValkeyInstance) (string, error) {
	return obj.AivenProject, nil
}
