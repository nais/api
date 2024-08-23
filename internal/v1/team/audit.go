package team

import (
	"github.com/nais/api/internal/v1/auditv1"
)

const (
	auditResourceTypeTeam auditv1.AuditResourceType = "TEAM"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeTeam, func(entry auditv1.GenericAuditEntry) auditv1.AuditEntry {
		switch entry.Action {
		case auditv1.AuditActionCreated:
			return TeamCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created team"),
			}
		default:
			return entry
		}
	})
}

type TeamCreatedAuditEntry struct {
	auditv1.GenericAuditEntry
}

type TeamUpdatedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data TeamUpdatedAuditEntryData `json:"data"`
}

type TeamUpdatedAuditEntryData struct {
	FieldsChanged []*TeamUpdatedAuditEntryDataUpdatedField `json:"updatedFields"`
}

type TeamUpdatedAuditEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}
