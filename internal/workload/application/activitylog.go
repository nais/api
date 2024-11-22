package application

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	auditResourceTypeApplication  activitylog.AuditResourceType = "APP"
	auditActionRestartApplication activitylog.AuditAction       = "RESTARTED"
)

func init() {
	activitylog.RegisterTransformer(auditResourceTypeApplication, func(entry activitylog.GenericAuditEntry) (activitylog.AuditEntry, error) {
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
			}, nil
		case activitylog.AuditActionDeleted:
			return ApplicationDeletedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Application deleted"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported application audit entry action: %q", entry.Action)
		}
	})
}

type ApplicationRestartedAuditEntry struct {
	activitylog.GenericAuditEntry
}

type ApplicationDeletedAuditEntry struct {
	activitylog.GenericAuditEntry
}
