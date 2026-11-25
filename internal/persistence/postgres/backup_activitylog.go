package postgres

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
)

const ActivityLogEntryResourceTypePostgresBackup activitylog.ActivityLogEntryResourceType = "POSTGRES_BACKUP"

type PostgresBackupActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypePostgresBackup, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			return PostgresBackupActivityLogEntry{entry.WithMessage("Requested a standalone Postgres backup")}, nil
		case activityLogEntryActionDeletionRequested:
			return PostgresBackupActivityLogEntry{entry.WithMessage("Requested deletion of a standalone Postgres backup")}, nil
		default:
			return nil, fmt.Errorf("unsupported Postgres backup activity log action: %q", entry.Action)
		}
	})
	activitylog.RegisterActivityType("POSTGRES_BACKUP_CREATED", activitylog.ActivityLogEntryActionCreated, ActivityLogEntryResourceTypePostgresBackup, "Triggered when a standalone Postgres backup is requested.")
	activitylog.RegisterActivityType("POSTGRES_BACKUP_DELETION_REQUESTED", activityLogEntryActionDeletionRequested, ActivityLogEntryResourceTypePostgresBackup, "Triggered when deletion of a standalone Postgres backup is requested.")
}

func logPostgresBackupChange(ctx context.Context, action activitylog.ActivityLogEntryAction, teamSlug slug.Slug, environment, name string) error {
	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          action,
		Actor:           authz.ActorFromContext(ctx).User,
		ResourceType:    ActivityLogEntryResourceTypePostgresBackup,
		ResourceName:    name,
		EnvironmentName: &environment,
		TeamSlug:        &teamSlug,
	})
}
