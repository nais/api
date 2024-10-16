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
			data, err := auditv1.TransformData(entry, func(data *SecretCreatedAuditEntryData) *SecretCreatedAuditEntryData {
				return data
			})
			if err != nil {
				return nil, err
			}
			return SecretCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Create secret"),
				Data:              data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported secret audit entry action: %q", entry.Action)
		}
	})
}

type SecretCreatedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data *SecretCreatedAuditEntryData `json:"data"`
}

type SecretCreatedAuditEntryData struct {
	Keys []string `json:"keys"`
}
