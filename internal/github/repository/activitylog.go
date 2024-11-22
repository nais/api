package repository

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	auditResourceTypeRepository activitylog.AuditResourceType = "REPOSITORY"
)

func init() {
	activitylog.RegisterTransformer(auditResourceTypeRepository, func(entry activitylog.GenericAuditEntry) (activitylog.AuditEntry, error) {
		switch entry.Action {
		case activitylog.AuditActionAdded:
			return RepositoryAddedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Added repository to team"),
			}, nil
		case activitylog.AuditActionRemoved:
			return RepositoryRemovedAuditEntry{
				GenericAuditEntry: entry.WithMessage("Removed repository from team"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported repository audit entry action: %q", entry.Action)
		}
	})
}

type RepositoryAddedAuditEntry struct {
	activitylog.GenericAuditEntry
}

type RepositoryRemovedAuditEntry struct {
	activitylog.GenericAuditEntry
}
