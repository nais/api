package reconciler

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogEntryResourceTypeReconciler    activitylog.ActivityLogEntryResourceType = "RECONCILER"
	activityLogEntryActionEnableReconciler    activitylog.ActivityLogEntryAction       = "ENABLE_RECONCILER"
	activityLogEntryActionDisableReconciler   activitylog.ActivityLogEntryAction       = "DISABLE_RECONCILER"
	activityLogEntryActionConfigureReconciler activitylog.ActivityLogEntryAction       = "CONFIGURE_RECONCILER"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogEntryResourceTypeReconciler, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogEntryActionEnableReconciler:
			return ReconcilerEnabledActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Enable reconciler"),
			}, nil
		case activityLogEntryActionDisableReconciler:
			return ReconcilerDisabledActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Disable reconciler"),
			}, nil
		case activityLogEntryActionConfigureReconciler:
			data, err := activitylog.TransformData(entry, func(data *ReconcilerConfiguredActivityLogData) *ReconcilerConfiguredActivityLogData {
				if len(data.UpdatedKeys) == 0 {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return ReconcilerConfiguredActivityLogEntry{
				GenericActivityLogEntry: entry.WithMessage("Configure reconciler"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported reconciler activity log entry action: %q", entry.Action)
		}
	})
}

type ReconcilerEnabledActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ReconcilerDisabledActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
}

type ReconcilerConfiguredActivityLogEntry struct {
	activitylog.GenericActivityLogEntry
	Data *ReconcilerConfiguredActivityLogData `json:"data"`
}

type ReconcilerConfiguredActivityLogData struct {
	UpdatedKeys []string `json:"updatedKeys"`
}