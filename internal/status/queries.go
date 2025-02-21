package status

import (
	"context"
	"slices"

	"github.com/nais/api/internal/workload"
	"github.com/sourcegraph/conc/pool"
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
		return int(a.GetLevel() - b.GetLevel())
	})

	return &WorkloadStatus{Errors: errs, State: state}
}

func ForWorkloads[T workload.Workload](ctx context.Context, workloads []T) []WorkloadStatusError {
	wg := pool.NewWithResults[[]WorkloadStatusError]().WithContext(ctx).WithMaxGoroutines(10)
	for _, workload := range workloads {
		wg.Go(func(ctx context.Context) ([]WorkloadStatusError, error) {
			return ForWorkload(ctx, workload).Errors, nil
		})
	}

	errors, _ := wg.Wait()
	var errs []WorkloadStatusError
	for _, e := range errors {
		errs = append(errs, e...)
	}
	return errs
}
