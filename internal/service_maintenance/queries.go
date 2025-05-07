package servicemaintenance

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/persistence/valkey"
)

func GetServiceMaintenances(ctx context.Context, valkey valkey.ValkeyInstance) (*ServiceMaintenance, error) {
	fmt.Printf("\n\n\n\n%v\n\n\n", valkey)
	key := aivenDataLoaderKey{
		project:     valkey.EnvironmentName,
		serviceName: valkey.Name,
	}
	res, err := ctx.Value(loadersKey).(*loaders).maintenanceLoader.Load(ctx, &key)
	if err != nil {
		return nil, err
	}
	return res, nil
}
