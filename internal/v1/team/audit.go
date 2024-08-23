package team

import "github.com/nais/api/internal/v1/auditv1"

const (
	auditResourceTypeTeam auditv1.AuditResourceType = "TEAM"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeTeam, func(entry auditv1.AuditLogGeneric) auditv1.AuditEntry {
		switch entry.Action {
		case auditv1.AuditActionCreated:
			return AuditLogTeamCreated{
				AuditLogGeneric: entry.WithMessage("Created team"),
			}
		default:
			return entry
		}
	})
}

type AuditLogTeamCreated struct {
	auditv1.AuditLogGeneric
}

type AuditLogTeamUpdated struct {
	auditv1.AuditLogGeneric
	Data AuditLogTeamUpdatedData `json:"data"`
}

type AuditLogTeamUpdatedData struct {
	FieldsChanged []*AuditLogTeamUpdatedFieldChange `json:"fieldsChanged"`
}

type AuditLogTeamUpdatedFieldChange struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}
