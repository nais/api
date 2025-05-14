package servicemaintenance

import (
	"context"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/valkey"
)

func StartValkeyMaintenance(ctx context.Context, input StartValkeyMaintenanceInput) error {
	valkeyInstance, err := valkey.Get(ctx, input.TeamSlug, input.EnvironmentName, input.ServiceName)
	if err != nil {
		return err
	}

	if err := fromContext(ctx).maintenanceMutator.aivenClient.ServiceMaintenanceStart(ctx, valkeyInstance.AivenProject, input.ServiceName); err != nil {
		fromContext(ctx).log.WithError(err).Error("Failed to start Valkey maintenance")
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionStartServiceMaintenance,
		ResourceType:    activityLogResourceTypeValkeyServiceMaintenance,
		TeamSlug:        &input.TeamSlug,
		EnvironmentName: &input.EnvironmentName,
		ResourceName:    input.ServiceName,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func StartOpenSearchMaintenance(ctx context.Context, input StartOpenSearchMaintenanceInput) error {
	instance, err := opensearch.Get(ctx, input.TeamSlug, input.EnvironmentName, input.ServiceName)
	if err != nil {
		return err
	}

	if err := fromContext(ctx).maintenanceMutator.aivenClient.ServiceMaintenanceStart(ctx, instance.AivenProject, input.ServiceName); err != nil {
		fromContext(ctx).log.WithError(err).Error("Failed to start OpenSearch maintenance")
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionStartServiceMaintenance,
		ResourceType:    activityLogResourceTypeOpenSearchServiceMaintenance,
		TeamSlug:        &input.TeamSlug,
		EnvironmentName: &input.EnvironmentName,
		ResourceName:    input.ServiceName,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func GetValkeyMaintenance(ctx context.Context, valkey valkey.ValkeyInstance) (*ValkeyMaintenance, error) {
	key := AivenDataLoaderKey{
		Project:     valkey.AivenProject,
		ServiceName: valkey.Name,
	}

	aivenMaintenance, err := fromContext(ctx).maintenanceLoader.Load(ctx, &key)
	if err != nil {
		return nil, err
	}

	updates := make([]*ValkeyMaintenanceUpdate, len(aivenMaintenance.Updates))
	for i, update := range aivenMaintenance.Updates {
		updates[i] = &ValkeyMaintenanceUpdate{
			Title:       *update.Description,
			Description: *update.Impact,
			StartAt:     update.StartAt,
		}

		if update.Deadline != nil {
			t, err := time.Parse(time.RFC3339, *update.Deadline)
			if err != nil {
				fromContext(ctx).log.WithError(err).Error("Failed to parse deadline")
				continue
			}

			updates[i].Deadline = &t
		}
	}

	return &ValkeyMaintenance{
		Updates: pagination.NewConnection(updates, nil, len(aivenMaintenance.Updates)),
	}, nil
}

func GetAivenMaintenance[MaintenanceType any, UpdateType any](ctx context.Context, key AivenDataLoaderKey) (*AivenMaintenance[MaintenanceType, UpdateType], error) {
	aivenMaintenance, err := fromContext(ctx).maintenanceLoader.Load(ctx, &key)
	if err != nil {
		return nil, err
	}

	updates := make([]*AivenUpdate[UpdateType], len(aivenMaintenance.Updates))
	for i, update := range aivenMaintenance.Updates {
		updates[i] = &AivenUpdate[UpdateType]{
			Title:       *update.Description,
			Description: *update.Impact,
			StartAt:     update.StartAt,
		}

		if update.Deadline != nil {
			t, err := time.Parse(time.RFC3339, *update.Deadline)
			if err != nil {
				fromContext(ctx).log.WithError(err).Error("Failed to parse deadline")
				continue
			}

			updates[i].Deadline = &t
		}
	}

	return &AivenMaintenance[MaintenanceType, UpdateType]{
		Updates: pagination.NewConnection(updates, nil, len(aivenMaintenance.Updates)),
	}, nil
}
