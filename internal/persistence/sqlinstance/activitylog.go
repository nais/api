package sqlinstance

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

	activitylog.RegisterFilter("POSTGRES_GRANT_ACCESS", activityLogEntryActionGrantAccess, activityLogEntryResourceTypePostgres)
}

type PostgresGrantAccessActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *PostgresGrantAccessActivityLogEntryData `json:"data"`
}

type PostgresGrantAccessActivityLogEntryData struct {
	Grantee string    `json:"grantee,string"`
	Until   time.Time `json:"until"`
}
