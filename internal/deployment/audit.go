package deployment

import (
	"fmt"

	"github.com/nais/api/internal/audit"
)

const (
	auditResourceTypeDeployKey audit.AuditResourceType = "DEPLOY_KEY"
)

func init() {
	audit.RegisterTransformer(auditResourceTypeDeployKey, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case audit.AuditActionUpdated:
			return TeamDeployKeyUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated deployment key"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported deploy key audit entry action: %q", entry.Action)
		}
	})
}

type TeamDeployKeyUpdatedAuditEntry struct {
	audit.GenericAuditEntry
}
