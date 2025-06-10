package status

import (
	"context"
	"time"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/job"
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
	runs, err := job.Runs(ctx, w.GetTeamSlug(), w.GetEnvironmentName(), w.GetName(), page)
	if err != nil {
		return &WorkloadStatusSynchronizationFailing{
			Level: WorkloadStatusErrorLevelUnknown,
		}
	}

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

	if tmpRun == nil {
		// No runs found, workload is not failing
		return nil
	}

	if tmpRun.Status().State == job.JobRunStateRunning {
		// Job is actively running
		return nil
	}

	if tmpRun.Failed {
		// Job run has failed
		return &WorkloadStatusFailedRun{
			Level:  WorkloadStatusErrorLevelWarning,
			Detail: tmpRun.Message,
			Name:   tmpRun.Name,
		}
	}

	return nil
}

func (checkJobRuns) Supports(w workload.Workload) bool {
	_, ok := w.(*job.Job)
	return ok
}
