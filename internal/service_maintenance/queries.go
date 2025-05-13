package servicemaintenance

import (
	"context"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/persistence/valkey"
)

func RunServiceMaintenance(ctx context.Context, service RunValkeyMaintenanceInput) error {
	err := ctx.Value(loadersKey).(*loaders).maintenanceMutator.client.ServiceMaintenanceStart(ctx, service.Project, service.ServiceName)
	if err != nil {
		return nil
	}
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionStartServiceMaintenance,
		ResourceType:    activityLogResourceTypeValkeyServiceMaintenance,
		TeamSlug:        &service.TeamSlug,
		EnvironmentName: &service.EnvironmentName,
		ResourceName:    service.ServiceName,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func GetServiceMaintenances(ctx context.Context, valkey valkey.ValkeyInstance) (*ServiceMaintenance, error) {
	key := AivenDataLoaderKey{
		project:     valkey.AivenProject,
		serviceName: valkey.Name,
	}
	return ctx.Value(loadersKey).(*loaders).maintenanceLoader.Load(ctx, &key)
}
