package team

import (
	"fmt"

	"github.com/nais/api/internal/v1/auditv1"
)

const (
	auditResourceTypeTeam auditv1.AuditResourceType = "TEAM"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeTeam, func(entry auditv1.GenericAuditEntry) (auditv1.AuditEntry, error) {
		switch entry.Action {
		case auditv1.AuditActionCreated:
			return TeamCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created team"),
			}, nil
		case auditv1.AuditActionUpdated:
			data, err := auditv1.TransformData(entry, func(data *TeamUpdatedAuditEntryData) *TeamUpdatedAuditEntryData {
				if len(data.UpdatedFields) == 0 {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return TeamUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated team"),
				Data:              data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported team audit entry action: %q", entry.Action)
		}
	})
}

type TeamCreatedAuditEntry struct {
	auditv1.GenericAuditEntry
}

type TeamUpdatedAuditEntry struct {
	auditv1.GenericAuditEntry
	Data *TeamUpdatedAuditEntryData `json:"data,omitempty"`
}

type TeamUpdatedAuditEntryData struct {
	UpdatedFields []*TeamUpdatedAuditEntryDataUpdatedField `json:"updatedFields,omitempty"`
}

type TeamUpdatedAuditEntryDataUpdatedField struct {
	Field    string  `json:"field"`
	OldValue *string `json:"oldValue,omitempty"`
	NewValue *string `json:"newValue,omitempty"`
}
