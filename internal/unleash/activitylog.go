package unleash

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
)

const (
	auditResourceTypeUnleash activitylog.AuditResourceType = "UNLEASH"
)

func init() {
	activitylog.RegisterTransformer(auditResourceTypeUnleash, func(entry activitylog.GenericAuditEntry) (activitylog.AuditEntry, error) {
		switch entry.Action {
		case activitylog.AuditActionCreated:
			return UnleashInstanceCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created Unleash instance"),
			}, nil
		case activitylog.AuditActionUpdated:
			data, err := activitylog.TransformData(entry, func(data *UnleashInstanceUpdatedAuditEntryData) *UnleashInstanceUpdatedAuditEntryData {
				if data.AllowedTeamSlug == nil && data.RevokedTeamSlug == nil {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return UnleashInstanceUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated Unleash instance"),
				Data:              data,
			}, nil

		default:
			return nil, fmt.Errorf("unsupported team audit entry action: %q", entry.Action)
		}
	})
}

type UnleashInstanceCreatedAuditEntry struct {
	activitylog.GenericAuditEntry
}

type UnleashInstanceUpdatedAuditEntry struct {
	activitylog.GenericAuditEntry
	Data *UnleashInstanceUpdatedAuditEntryData `json:"data"`
}

type UnleashInstanceUpdatedAuditEntryData struct {
	RevokedTeamSlug *slug.Slug `json:"revokedTeamSlug"`
	AllowedTeamSlug *slug.Slug `json:"allowedTeamSlug"`
}
