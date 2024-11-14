package repository

import (
	"fmt"

	"github.com/nais/api/internal/audit"
)

const (
	auditResourceTypeRepository audit.AuditResourceType = "REPOSITORY"
)

func init() {
	audit.RegisterTransformer(auditResourceTypeRepository, func(entry audit.GenericAuditEntry) (audit.AuditEntry, error) {
		switch entry.Action {
		case audit.AuditActionAdded:
			return RepositoryAddedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Added repository to team"),
			}, nil
		case audit.AuditActionRemoved:
			return RepositoryRemovedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Removed repository from team"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported repository audit entry action: %q", entry.Action)
		}
	})
}

type RepositoryAddedAuditEntry struct {
	audit.GenericAuditEntry
}

type RepositoryRemovedAuditEntry struct {
	audit.GenericAuditEntry
}
