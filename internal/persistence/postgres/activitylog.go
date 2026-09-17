package postgres

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryActionGrantAccess                 activitylog.ActivityLogEntryAction = "GRANT_ACCESS"
	activityLogEntryActionCreatePersonalAccess        activitylog.ActivityLogEntryAction = "CREATE_PERSONAL_ACCESS"
	activityLogEntryActionGetPersonalAccessConnection activitylog.ActivityLogEntryAction = "GET_PERSONAL_ACCESS_CONNECTION"

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
		case activityLogEntryActionCreatePersonalAccess:
			data, err := activitylog.UnmarshalData[PostgresPersonalAccessCreatedActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming postgres personal access activity log entry data: %w", err)
			}
			return PostgresPersonalAccessCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Created personal Postgres access for %s until %s", data.Username, data.ExpiresAt)),
				Data:                    data,
			}, nil
		case activityLogEntryActionGetPersonalAccessConnection:
			return PostgresPersonalAccessConnectionActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Retrieved personal Postgres connection materials"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported postgres activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("POSTGRES_GRANT_ACCESS", activityLogEntryActionGrantAccess, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_PERSONAL_ACCESS_CREATED", activityLogEntryActionCreatePersonalAccess, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_PERSONAL_ACCESS_CONNECTION", activityLogEntryActionGetPersonalAccessConnection, activityLogEntryResourceTypePostgres)
	activitylog.RegisterFilter("POSTGRES_DELETED", activitylog.ActivityLogEntryActionDeleted, activityLogEntryResourceTypePostgres)
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

type PostgresPersonalAccessCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *PostgresPersonalAccessCreatedActivityLogEntryData `json:"data"`
}

type PostgresPersonalAccessCreatedActivityLogEntryData struct {
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expiresAt"`
	Reason    string    `json:"reason"`
}

type PostgresPersonalAccessConnectionActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type PostgresPersonalAccessConnectionActivityLogEntryData struct{}
