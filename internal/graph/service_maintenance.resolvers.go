package graph

import (
	"context"

	"github.com/nais/api/internal/persistence/valkey"
	servicemaintenance "github.com/nais/api/internal/service_maintenance"
)

func (r *valkeyInstanceResolver) Maintenance(ctx context.Context, obj *valkey.ValkeyInstance) (*servicemaintenance.ServiceMaintenance, error) {
	return servicemaintenance.GetServiceMaintenances(ctx, *obj)
}
