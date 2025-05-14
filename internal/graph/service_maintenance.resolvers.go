package graph

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/valkey"
	servicemaintenance "github.com/nais/api/internal/service_maintenance"
)

func (r *mutationResolver) StartValkeyMaintenance(ctx context.Context, input servicemaintenance.StartValkeyMaintenanceInput) (*servicemaintenance.StartValkeyMaintenancePayload, error) {
	if err := authz.CanStartServiceMaintenance(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := servicemaintenance.StartValkeyMaintenance(ctx, input); err != nil {
		return nil, err
	}

	return &servicemaintenance.StartValkeyMaintenancePayload{
		Error: new(string),
	}, nil
}

func (r *mutationResolver) StartOpenSearchMaintenance(ctx context.Context, input servicemaintenance.StartOpenSearchMaintenanceInput) (*servicemaintenance.StartOpenSearchMaintenancePayload, error) {
	if err := authz.CanStartServiceMaintenance(ctx, input.TeamSlug); err != nil {
		return nil, err
	}

	if err := servicemaintenance.StartOpenSearchMaintenance(ctx, input); err != nil {
		return nil, err
	}

	return &servicemaintenance.StartOpenSearchMaintenancePayload{
		Error: new(string),
	}, nil
}

func (r *openSearchResolver) Maintenance(ctx context.Context, obj *opensearch.OpenSearch) (*servicemaintenance.OpenSearchMaintenance, error) {
	return servicemaintenance.GetAivenMaintenance[servicemaintenance.OpenSearchMaintenance, servicemaintenance.OpenSearchMaintenanceUpdate](ctx, servicemaintenance.AivenDataLoaderKey{
		Project:     obj.AivenProject,
		ServiceName: obj.Name,
	})
}

func (r *valkeyInstanceResolver) Maintenance(ctx context.Context, obj *valkey.ValkeyInstance) (*servicemaintenance.ValkeyMaintenance, error) {
	return servicemaintenance.GetValkeyMaintenance(ctx, *obj)
}
