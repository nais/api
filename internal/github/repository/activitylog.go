package repository

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	activityLogResourceTypeRepository activitylog.ActivityLogResourceType = "REPOSITORY"
)

func init() {
	activitylog.RegisterTransformer(activityLogResourceTypeRepository, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activitylog.ActivityLogActionAdded:
			return RepositoryAddedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Added repository to team"),
			}, nil
		case activitylog.ActivityLogActionRemoved:
			return RepositoryRemovedActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Removed repository from team"),
			}, nil

		default:
			return nil, fmt.Errorf("unsupported repository activity log entry action: %q", entry.Action)
		}
	})
}

type RepositoryAddedActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type RepositoryRemovedActivityLog struct {
	activitylog.GenericActivityLogEntry
}
