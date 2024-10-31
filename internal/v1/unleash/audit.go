package unleash

import (
	"fmt"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1"
)

const (
	auditResourceTypeUnleash auditv1.AuditResourceType = "UNLEASH"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeUnleash, func(entry auditv1.GenericAuditEntry) (auditv1.AuditEntry, error) {
		switch entry.Action {
		case auditv1.AuditActionCreated:
			data, err := auditv1.TransformData(entry, func(data *UnleashInstanceCreatedAuditEntryData) *UnleashInstanceCreatedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}

			return UnleashInstanceCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created Unleash instance"),
				Data:              data,
			}, nil
		case auditv1.AuditActionUpdated:
			data, err := auditv1.TransformData(entry, func(data *UnleashInstanceUpdatedAuditEntryData) *UnleashInstanceUpdatedAuditEntryData {
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
	auditv1.GenericAuditEntry
	Data *UnleashInstanceCreatedAuditEntryData `json:"data"`
}

type UnleashInstanceCreatedAuditEntryData struct {
	Name string `json:"name"`
}

type UnleashInstanceUpdatedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data *UnleashInstanceUpdatedAuditEntryData `json:"data"`
}

type UnleashInstanceUpdatedAuditEntryData struct {
	RevokedTeamSlug *slug.Slug `json:"revokedTeamSlug"`
	AllowedTeamSlug *slug.Slug `json:"allowedTeamSlug"`
}
