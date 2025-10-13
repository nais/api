package servicemaintenance

import (
	"context"
	"strings"
	"time"

	aiven_service "github.com/aiven/go-client-codegen/handler/service"
	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/persistence/valkey"
	servicemaintenanceal "github.com/nais/api/internal/servicemaintenance/activitylog"
)

func StartValkeyMaintenance(ctx context.Context, input StartValkeyMaintenanceInput) error {
	vk, err := valkey.Get(ctx, input.TeamSlug, input.EnvironmentName, input.ServiceName)
	if err != nil {
		return err
	}

	if err := fromContext(ctx).maintenanceMutator.aivenClient.ServiceMaintenanceStart(ctx, vk.AivenProject, vk.FullyQualifiedName()); err != nil {
		fromContext(ctx).log.WithError(err).Error("Failed to start Valkey maintenance")
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          servicemaintenanceal.ActivityLogEntryActionMaintenanceStarted,
		ResourceType:    valkey.ActivityLogEntryResourceTypeValkey,
		TeamSlug:        &vk.TeamSlug,
		EnvironmentName: &vk.EnvironmentName,
		ResourceName:    vk.Name,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func StartOpenSearchMaintenance(ctx context.Context, input StartOpenSearchMaintenanceInput) error {
	instance, err := opensearch.Get(ctx, input.TeamSlug, input.EnvironmentName, input.ServiceName)
	if err != nil {
		return err
	}

	if err := fromContext(ctx).maintenanceMutator.aivenClient.ServiceMaintenanceStart(ctx, instance.AivenProject, instance.FullyQualifiedName()); err != nil {
		fromContext(ctx).log.WithError(err).Error("Failed to start OpenSearch maintenance")
		return err
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          servicemaintenanceal.ActivityLogEntryActionMaintenanceStarted,
		ResourceType:    opensearch.ActivityLogEntryResourceTypeOpenSearch,
		TeamSlug:        &instance.TeamSlug,
		EnvironmentName: &instance.EnvironmentName,
		ResourceName:    instance.Name,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func GetAivenMaintenanceWindow(ctx context.Context, key AivenDataLoaderKey) (*MaintenanceWindow, error) {
	windowFromAiven, err := fromContext(ctx).maintenanceLoader.Load(ctx, &key)
	if err != nil {
		return nil, err
	}

	if windowFromAiven.Dow == aiven_service.MaintenanceDowTypeNever {
		return nil, nil
	}

	parsedTime, err := time.Parse(time.TimeOnly, windowFromAiven.Time)
	if err != nil {
		return nil, err
	}

	parsedTimeAsString := parsedTime.Format(time.TimeOnly)
	return &MaintenanceWindow{
		DayOfWeek: model.Weekday(strings.ToUpper(string(windowFromAiven.Dow))),
		TimeOfDay: parsedTimeAsString,
	}, nil
}

func GetAivenMaintenanceUpdates[UpdateType OpenSearchMaintenanceUpdate | ValkeyMaintenanceUpdate](ctx context.Context, key AivenDataLoaderKey) ([]*UpdateType, error) {
	updatesFromAiven, err := fromContext(ctx).maintenanceLoader.Load(ctx, &key)
	if err != nil {
		return nil, err
	}

	updates := make([]*UpdateType, len(updatesFromAiven.Updates))
	for i, update := range updatesFromAiven.Updates {
		au := &AivenUpdate{
			Title:   *update.Description,
			StartAt: update.StartAt,
		}

		if update.Impact != nil {
			au.Description = *update.Impact
		}

		if update.Deadline != nil {
			t, err := time.Parse(time.RFC3339, *update.Deadline)
			if err != nil {
				fromContext(ctx).log.WithError(err).Error("Failed to parse deadline")
				continue
			}

			au.Deadline = &t
		}
		updates[i] = &UpdateType{au}
	}

	return updates, nil
}
