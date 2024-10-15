package status

import (
	"context"
	"time"

	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/job"
	"k8s.io/utils/ptr"
)

type checkJobRuns struct{}

func (c checkJobRuns) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	s := c.run(ctx, w)
	if s == nil {
		return nil, WorkloadStateNais
	}
	return []WorkloadStatusError{s}, WorkloadStateFailing
}

func (checkJobRuns) run(ctx context.Context, w workload.Workload) WorkloadStatusError {
	page, _ := pagination.ParsePage(ptr.To(5), nil, nil, nil)
	runs, _ := job.Runs(ctx, w.GetTeamSlug(), w.GetName(), page)

	var tmpTime time.Time
	var tmpRun *job.JobRun
	for _, run := range runs.Nodes() {
		if run.StartTime != nil && run.StartTime.After(tmpTime) {
			tmpTime = *run.StartTime
			tmpRun = run
		} else {
			continue
		}
	}

	if tmpRun != nil {
		if tmpRun.Failed {
			return &WorkloadStatusFailedRun{
				Level:  WorkloadStatusErrorLevelWarning,
				Detail: tmpRun.Message,
				Name:   tmpRun.Name,
			}
		}
	}

	return nil
}

func (checkJobRuns) Supports(w workload.Workload) bool {
	_, ok := w.(*job.Job)
	return ok
}
