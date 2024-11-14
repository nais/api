package job

import (
	"fmt"

	"github.com/nais/api/internal/audit"
)

const (
	auditResourceTypeJob  audit.AuditResourceType = "JOB"
	auditActionTriggerJob audit.AuditAction       = "TRIGGER_JOB"
)

func init() {
	audit.RegisterTransformer(auditResourceTypeJob, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case auditActionTriggerJob:
			return JobTriggeredAuditEntry{
				GenericAuditEntry: entry.WithMessage("Job triggered"),
			}, nil
		case audit.AuditActionDeleted:
			return JobDeletedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Job deleted"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported job audit entry action: %q", entry.Action)
		}
	})
}

type JobTriggeredAuditEntry struct {
	audit.GenericAuditEntry
}

type JobDeletedAuditEntry struct {
	audit.GenericAuditEntry
}
