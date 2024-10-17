package secret

import (
	"fmt"

	"github.com/nais/api/internal/v1/auditv1"
)

const (
	auditResourceTypeSecret auditv1.AuditResourceType = "SECRET"
	auditActionCreateSecret auditv1.AuditAction       = "CREATE_SECRET"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeSecret, func(entry auditv1.GenericAuditEntry) (auditv1.AuditEntry, error) {
		switch entry.Action {
		case auditActionCreateSecret:
			return SecretCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Create secret"),
			}, nil
		default:
			return nil, fmt.Errorf("unsupported secret audit entry action: %q", entry.Action)
		}
	})
}

type SecretCreatedAuditEntry struct {
	auditv1.GenericAuditEntry
}
