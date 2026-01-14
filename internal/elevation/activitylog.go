package elevation

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeElevation activitylog.ActivityLogEntryResourceType = "ELEVATION"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeElevation, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionCreated:
			data, err := activitylog.UnmarshalData[ElevationCreatedActivityLogEntryData](entry)
			if err != nil {
				return nil, err
			}

			return ElevationCreatedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage(fmt.Sprintf("Created elevation for %s access to %s", data.ElevationType, data.TargetResourceName)),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported elevation activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("ELEVATION_CREATED", activitylog.ActivityLogEntryActionCreated, activityLogEntryResourceTypeElevation)
}

type ElevationCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ElevationCreatedActivityLogEntryData `json:"data"`
}

func (ElevationCreatedActivityLogEntry) IsActivityLogEntry() {}

type ElevationCreatedActivityLogEntryData struct {
	ElevationType      ElevationType `json:"elevationType"`
	TargetResourceName string        `json:"targetResourceName"`
	Reason             string        `json:"reason"`
	ExpiresAt          time.Time     `json:"expiresAt"`
}
