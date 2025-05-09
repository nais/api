package servicemaintenance

import (
	"context"

	"github.com/nais/api/internal/persistence/valkey"
)

func RunServiceMaintenance(ctx context.Context, service RunMaintenanceInput) error {
	return ctx.Value(loadersKey).(*loaders).maintenanceMutator.client.ServiceMaintenanceStart(ctx, service.Project, service.ServiceName)
}

func GetServiceMaintenances(ctx context.Context, valkey valkey.ValkeyInstance) (*ServiceMaintenance, error) {
	key := AivenDataLoaderKey{
		project:     valkey.AivenProject,
		serviceName: valkey.Name,
	}
	return ctx.Value(loadersKey).(*loaders).maintenanceLoader.Load(ctx, &key)
}
