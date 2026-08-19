package postgres

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryActionGrantAccess activitylog.ActivityLogEntryAction = "GRANT_ACCESS"

	activityLogEntryResourceTypePostgres activitylog.ActivityLogEntryResourceType = "POSTGRES"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypePostgres, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionDeleted:
			return PostgresDeletedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Deleted Postgres"),
			}, nil
		case activityLogEntryActionGrantAccess:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for postgres grant access activity log entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for postgres grant access activity log entry")
			}
			data, err := activitylog.UnmarshalData[PostgresGrantAccessActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming postgres grant access activity log entry data: %w", err)
			}
			return PostgresGrantAccessActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Granted access to %s until %s", data.Grantee, data.Until)),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported postgres activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterActivityType("POSTGRES_GRANT_ACCESS",
		activityLogEntryActionGrantAccess,
		activityLogEntryResourceTypePostgres,
		activitylog.WithDescription("Triggered when user access to a Postgres instance is granted."),
	)
	activitylog.RegisterActivityType("POSTGRES_DELETED",
		activitylog.ActivityLogEntryActionDeleted,
		activityLogEntryResourceTypePostgres,
		activitylog.WithDescription("Triggered when a Postgres instance is deleted."),
	)
}

type PostgresDeletedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type PostgresGrantAccessActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *PostgresGrantAccessActivityLogEntryData `json:"data"`
}

type PostgresGrantAccessActivityLogEntryData struct {
	Grantee string    `json:"grantee,string"`
	Until   time.Time `json:"until"`
}
