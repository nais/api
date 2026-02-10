package zalandopostgres

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryActionGrantAccess activitylog.ActivityLogEntryAction = "GRANT_ACCESS"

	activityLogEntryResourceTypeZalandoPostgres activitylog.ActivityLogEntryResourceType = "ZALANDO_POSTGRES"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeZalandoPostgres, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionGrantAccess:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for zalando postgres grant access activity log entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for zalando postgres grant access activity log entry")
			}
			data, err := activitylog.UnmarshalData[ZalandoPostgresGrantAccessActivityLogEntryData](entry)
			if err != nil {
				return nil, fmt.Errorf("transforming zalando postgres grant access activity log entry data: %w", err)
			}
			return ZalandoPostgresGrantAccessActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Granted access to %s until %s", data.Grantee, data.Until)),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported zalando postgres activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("ZALANDO_POSTGRES_GRANT_ACCESS", activityLogEntryActionGrantAccess, activityLogEntryResourceTypeZalandoPostgres)
}

type ZalandoPostgresGrantAccessActivityLogEntry struct {
	activitylog.GenericActivityLogEntry

	Data *ZalandoPostgresGrantAccessActivityLogEntryData `json:"data"`
}

type ZalandoPostgresGrantAccessActivityLogEntryData struct {
	Grantee string    `json:"grantee,string"`
	Until   time.Time `json:"until"`
}
