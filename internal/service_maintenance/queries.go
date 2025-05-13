package servicemaintenance

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
)

func RunServiceMaintenance(ctx context.Context, service RunMaintenanceInput) error {
	var resourceType activitylog.ActivityLogEntryResourceType
	switch service.ServiceType {
	case ServiceTypeValkey:
		resourceType = activityLogResourceTypeValkeyServiceMaintenance
	case ServiceTypeOpensearch:
		resourceType = activityLogResourceTypeOpenSearchServiceMaintenance
	default:
		return fmt.Errorf("unknown instance type %s", service.ServiceType)
	}

	err := ctx.Value(loadersKey).(*loaders).maintenanceMutator.client.ServiceMaintenanceStart(ctx, service.Project, service.ServiceName)
	if err != nil {
		return nil
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionStartServiceMaintenance,
		ResourceType:    resourceType,
		TeamSlug:        &service.TeamSlug,
		EnvironmentName: &service.EnvironmentName,
		ResourceName:    service.ServiceName,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func GetServiceMaintenances(ctx context.Context, key AivenDataLoaderKey) (*ServiceMaintenance, error) {
	return ctx.Value(loadersKey).(*loaders).maintenanceLoader.Load(ctx, &key)
}
