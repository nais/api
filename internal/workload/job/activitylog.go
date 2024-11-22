package job

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	auditResourceTypeJob  activitylog.AuditResourceType = "JOB"
	auditActionTriggerJob activitylog.AuditAction       = "TRIGGER_JOB"
)

func init() {
	activitylog.RegisterTransformer(auditResourceTypeJob, func(entry activitylog.GenericAuditEntry) (activitylog.AuditEntry, error) {
		switch entry.Action {
		case auditActionTriggerJob:
			return JobTriggeredAuditEntry{
				GenericAuditEntry: entry.WithMessage("Job triggered"),
			}, nil
		case activitylog.AuditActionDeleted:
			return JobDeletedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Job deleted"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported job audit entry action: %q", entry.Action)
		}
	})
}

type JobTriggeredAuditEntry struct {
	activitylog.GenericAuditEntry
}

type JobDeletedAuditEntry struct {
	activitylog.GenericAuditEntry
}
