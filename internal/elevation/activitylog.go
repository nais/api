package elevation

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeElevation activitylog.ActivityLogEntryResourceType = "ELEVATION"
	activityLogEntryActionRevoked         activitylog.ActivityLogEntryAction       = "REVOKED"
	activityLogEntryActionExpired         activitylog.ActivityLogEntryAction       = "EXPIRED"
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
		case activityLogEntryActionRevoked:
			return ElevationRevokedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Revoked elevation"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported elevation activity log entry action: %q", entry.Action)
		}
	})

	activitylog.RegisterFilter("ELEVATION_CREATED", activitylog.ActivityLogEntryActionCreated, activityLogEntryResourceTypeElevation)
	activitylog.RegisterFilter("ELEVATION_REVOKED", activityLogEntryActionRevoked, activityLogEntryResourceTypeElevation)
	activitylog.RegisterFilter("ELEVATION_EXPIRED", activityLogEntryActionExpired, activityLogEntryResourceTypeElevation)
}

type ElevationCreatedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ElevationCreatedActivityLogEntryData `json:"data"`
}

func (ElevationCreatedActivityLogEntry) IsActivityLogEntry() {}

type ElevationCreatedActivityLogEntryData struct {
	ElevationType      string `json:"elevationType"`
	TargetResourceName string `json:"targetResourceName"`
	Reason             string `json:"reason"`
	ExpiresAt          string `json:"expiresAt"`
}

func (e ElevationCreatedActivityLogEntry) GetElevationType() string {
	if e.Data == nil {
		return ""
	}
	return e.Data.ElevationType
}

func (e ElevationCreatedActivityLogEntry) GetTargetResourceName() string {
	if e.Data == nil {
		return ""
	}
	return e.Data.TargetResourceName
}

func (e ElevationCreatedActivityLogEntry) GetReason() string {
	if e.Data == nil {
		return ""
	}
	return e.Data.Reason
}

func (e ElevationCreatedActivityLogEntry) GetExpiresAt() string {
	if e.Data == nil {
		return ""
	}
	return e.Data.ExpiresAt
}

type ElevationRevokedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

func (ElevationRevokedActivityLogEntry) IsActivityLogEntry() {}
