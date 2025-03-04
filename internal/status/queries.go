package status

import (
	"context"
	"slices"

	"github.com/nais/api/internal/workload"
)

func ForWorkload(ctx context.Context, w workload.Workload) *WorkloadStatus {
	var errs []WorkloadStatusError
	state := WorkloadStateNais
	for _, check := range checksToRun {
		if check.Supports(w) {
			v, s := check.Run(ctx, w)
			if len(v) == 0 {
				continue
			}
			if s > state {
				state = s
			}
			errs = append(errs, v...)
		}
	}

	slices.SortFunc(errs, func(a, b WorkloadStatusError) int {
		return int(b.GetLevel() - a.GetLevel())
	})

	return &WorkloadStatus{Errors: errs, State: state}
}
