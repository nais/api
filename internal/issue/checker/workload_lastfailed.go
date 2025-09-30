package checker

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/workload/job"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func (w Workload) failedJobRuns(name, team, env string) *Issue {
	lastRun, err := w.lastRun(name, team, env)
	if err != nil {
		w.log.WithError(err).Error("fetch last job run")
		return nil
	}

	if lastRun == nil {
		return nil
	}

	if lastRun.Status().State == job.JobRunStateRunning {
		return nil
	}

	if lastRun.Failed {
		return &Issue{
			IssueType:    issue.IssueTypeFailedJobRuns,
			ResourceName: name,
			ResourceType: issue.ResourceTypeJob,
			Team:         team,
			Env:          env,
			Severity:     issue.SeverityWarning,
			Message:      fmt.Sprintf("Job has failing runs. Last run '%s' failed with message: %s", lastRun.Name, lastRun.Message),
		}
	}

	return nil
}

func (w Workload) lastRun(jobName, team, env string) (*job.JobRun, error) {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{jobName})
	if err != nil {
		return nil, fmt.Errorf("create label requirement: %w", err)
	}

	selector := labels.NewSelector().Add(*nameReq)
	runs := w.RunWatcher.GetByNamespace(team, watcher.InCluster(env), watcher.WithLabels(selector))

	var latestTime time.Time
	var latest *job.JobRun

	for _, run := range runs {
		j := job.ToGraphJobRun(run.Obj, env)
		if j.StartTime != nil && j.StartTime.After(latestTime) {
			latestTime = *j.StartTime
			latest = j
		}
	}
	return latest, nil
}
