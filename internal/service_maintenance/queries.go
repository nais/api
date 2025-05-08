package servicemaintenance

import (
	"context"

	"github.com/nais/api/internal/persistence/valkey"
)

func GetServiceMaintenances(ctx context.Context, valkey valkey.ValkeyInstance) (*ServiceMaintenance, error) {
	key := aivenDataLoaderKey{
		project:     valkey.AivenProject,
		serviceName: valkey.Name,
	}
	return ctx.Value(loadersKey).(*loaders).maintenanceLoader.Load(ctx, &key)
}
