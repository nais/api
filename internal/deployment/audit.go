package deployment

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	auditResourceTypeDeployKey activitylog.AuditResourceType = "DEPLOY_KEY"
)

func init() {
	activitylog.RegisterTransformer(auditResourceTypeDeployKey, func(entry activitylog.GenericAuditEntry) (activitylog.AuditEntry, error) {
		switch entry.Action {
		case activitylog.AuditActionUpdated:
			return TeamDeployKeyUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated deployment key"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported deploy key audit entry action: %q", entry.Action)
		}
	})
}

type TeamDeployKeyUpdatedAuditEntry struct {
	activitylog.GenericAuditEntry
}
