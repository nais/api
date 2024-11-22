package reconciler

import (
	"fmt"

	"github.com/nais/api/internal/activitylog"
)

const (
	ActivityLogResourceTypeReconciler    activitylog.ActivityLogResourceType = "RECONCILER"
	activityLogActionEnableReconciler    activitylog.ActivityLogAction       = "ENABLE_RECONCILER"
	activityLogActionDisableReconciler                                       = "DISABLE_RECONCILER"
	activityLogActionConfigureReconciler                                     = "CONFIGURE_RECONCILER"
)

func init() {
	activitylog.RegisterTransformer(ActivityLogResourceTypeReconciler, func(entry activitylog.GenericActivityLogEntry) (activitylog.ActivityLogEntry, error) {
		switch entry.Action {
		case activityLogActionEnableReconciler:
			return ReconcilerEnabledActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Enable reconciler"),
			}, nil
		case activityLogActionDisableReconciler:
			return ReconcilerDisabledActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Disable reconciler"),
			}, nil
		case activityLogActionConfigureReconciler:
			data, err := activitylog.TransformData(entry, func(data *ReconcilerConfiguredActivityLogData) *ReconcilerConfiguredActivityLogData {
				if len(data.UpdatedKeys) == 0 {
					return nil
				}
				return data
			})
			if err != nil {
				return nil, err
			}

			return ReconcilerConfiguredActivityLog{
				GenericActivityLogEntry: entry.WithMessage("Configure reconciler"),
				Data:                    data,
			}, nil
		default:
			return nil, fmt.Errorf("unsupported reconciler activity log entry action: %q", entry.Action)
		}
	})
}

type ReconcilerEnabledActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type ReconcilerDisabledActivityLog struct {
	activitylog.GenericActivityLogEntry
}

type ReconcilerConfiguredActivityLog struct {
	activitylog.GenericActivityLogEntry
	Data *ReconcilerConfiguredActivityLogData `json:"data"`
}

type ReconcilerConfiguredActivityLogData struct {
	UpdatedKeys []string `json:"updatedKeys"`
}
