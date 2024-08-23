package team

import (
	"bytes"
	"encoding/json"

	"github.com/nais/api/internal/v1/auditv1"
)

const (
	auditResourceTypeTeam auditv1.AuditResourceType = "TEAM"
)

func init() {
	auditv1.RegisterTransformer(auditResourceTypeTeam, func(entry auditv1.GenericAuditEntry) auditv1.AuditEntry {
		// TODO: return error instead of panicking
		switch entry.Action {
		case auditv1.AuditActionCreated:
			return TeamCreatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Created team"),
			}
		case auditv1.AuditActionUpdated:
			data := TeamUpdatedAuditEntryData{}
			if err := json.NewDecoder(bytes.NewReader(entry.Data)).Decode(&data); err != nil {
				panic("failed to decode data associated with audit entry")
			}
			return TeamUpdatedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Updated team"),
				Data: func(data TeamUpdatedAuditEntryData) *TeamUpdatedAuditEntryData {
					if len(data.UpdatedFields) == 0 {
						return nil
					}
					return &data
				}(data),
			}
		default:
			panic("unsupported team audit entry action: " + entry.Action)
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
