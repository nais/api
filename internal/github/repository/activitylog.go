package repository

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogEntryResourceTypeRepository activitylog.ActivityLogEntryResourceType = "REPOSITORY"
)

func init() {
	activitylog.RegisterTransformer(activityLogEntryResourceTypeRepository, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogEntryActionAdded:
			return RepositoryAddedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Added repository to team"),
			}, nil
		case activitylog.ActivityLogEntryActionRemoved:
			return RepositoryRemovedActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Removed repository from team"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported repository activity log entry action: %q", entry.Action)
		}
	})
}

type RepositoryAddedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type RepositoryRemovedActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}
