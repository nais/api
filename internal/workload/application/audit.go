package application

import (
	"fmt"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/slug"
)

const (
	auditResourceTypeApplication  audit.AuditResourceType = "APP"
	auditActionRestartApplication audit.AuditAction       = "RESTARTED"
)

func init() {
	audit.RegisterTransformer(auditResourceTypeApplication, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case auditActionRestartApplication:
			if entry.TeamSlug == nil {
				return nil, fmt.Errorf("missing team slug for application delete audit entry")
			}
			if entry.EnvironmentName == nil {
				return nil, fmt.Errorf("missing environment name for application delete audit entry")
			}
			return ApplicationRestartedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Application restarted"),
				Data: &ApplicationRestartedAuditEntryData{
					ApplicationName: entry.ResourceName,
					TeamSlug:        slug.Slug(*entry.TeamSlug),
					EnvironmentName: *entry.EnvironmentName,
				},
			}, nil
		case audit.AuditActionDeleted:
			return ApplicationDeletedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Application deleted"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported application audit entry action: %q", entry.Action)
		}
	})
}

type ApplicationRestartedAuditEntry struct {
	audit.GenericAuditEntry
	Data *ApplicationRestartedAuditEntryData
}

type ApplicationDeletedAuditEntry struct {
	audit.GenericAuditEntry
}

type ApplicationRestartedAuditEntryData struct {
	ApplicationName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}
