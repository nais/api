package unleash

import (
	"fmt"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/slug"
)

const (
	auditResourceTypeUnleash audit.AuditResourceType = "UNLEASH"
)

func init() {
	audit.RegisterTransformer(auditResourceTypeUnleash, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case audit.AuditActionCreated:
			data, err := audit.TransformData(entry, func(data *UnleashInstanceCreatedAuditEntryData) *UnleashInstanceCreatedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return UnleashInstanceCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created Unleash instance"),
				Data:              data,
			}, nil
		case audit.AuditActionUpdated:
			data, err := audit.TransformData(entry, func(data *UnleashInstanceUpdatedAuditEntryData) *UnleashInstanceUpdatedAuditEntryData {
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
	audit.GenericAuditEntry
	Data *UnleashInstanceCreatedAuditEntryData `json:"data"`
}

type UnleashInstanceCreatedAuditEntryData struct {
	Name string `json:"name"`
}

type UnleashInstanceUpdatedAuditEntry struct {
	audit.GenericAuditEntry
	Data *UnleashInstanceUpdatedAuditEntryData `json:"data"`
}

type UnleashInstanceUpdatedAuditEntryData struct {
	RevokedTeamSlug *slug.Slug `json:"revokedTeamSlug"`
	AllowedTeamSlug *slug.Slug `json:"allowedTeamSlug"`
}
