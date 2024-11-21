package reconciler

import (
	"fmt"

	"github.com/nais/api/internal/audit"
)

const (
	AuditResourceTypeReconciler    audit.AuditResourceType = "RECONCILER"
	auditActionEnableReconciler    audit.AuditAction       = "ENABLE_RECONCILER"
	auditActionDisableReconciler                           = "DISABLE_RECONCILER"
	auditActionConfigureReconciler                         = "CONFIGURE_RECONCILER"
)

func init() {
	audit.RegisterTransformer(AuditResourceTypeReconciler, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case auditActionEnableReconciler:
			return ReconcilerEnabledAuditEntry{
				GenericAuditEntry: entry.WithMessage("Enable reconciler"),
			}, nil
		case auditActionDisableReconciler:
			return ReconcilerDisabledAuditEntry{
				GenericAuditEntry: entry.WithMessage("Disable reconciler"),
			}, nil
		case auditActionConfigureReconciler:
			data, err := audit.TransformData(entry, func(data *ReconcilerConfiguredAuditEntryData) *ReconcilerConfiguredAuditEntryData {
				if len(data.UpdatedKeys) == 0 {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return ReconcilerConfiguredAuditEntry{
				GenericAuditEntry: entry.WithMessage("Configure reconciler"),
				Data:              data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported reconciler audit entry action: %q", entry.Action)
		}
	})
}

type ReconcilerEnabledAuditEntry struct {
	audit.GenericAuditEntry
}

type ReconcilerDisabledAuditEntry struct {
	audit.GenericAuditEntry
}

type ReconcilerConfiguredAuditEntry struct {
	audit.GenericAuditEntry
	Data *ReconcilerConfiguredAuditEntryData `json:"data"`
}

type ReconcilerConfiguredAuditEntryData struct {
	UpdatedKeys []string `json:"updatedKeys"`
}
