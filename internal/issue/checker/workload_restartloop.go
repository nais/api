package checker

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/issue"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	v1 "k8s.io/api/core/v1"
)

const (
	restartLoopWarningMinRestarts  = 3
	restartLoopCriticalMinRestarts = 10
	restartLoopWarningWindow       = 30 * time.Minute
	restartLoopCriticalWindow      = 10 * time.Minute
)

// restartLoop checks whether an application is stuck in a restart loop.
// It returns a Warning issue if any pod has restarted ≥3 times within the last 30 minutes,
// or a Critical issue if any pod has restarted ≥10 times within the last 10 minutes.
// pods must already be fetched by the caller (e.g. Run).
func (w Workload) restartLoop(app *nais_io_v1alpha1.Application, pods []*v1.Pod, team, env string) *Issue {
	now := time.Now()

	type candidate struct {
		restartCount      int32
		lastExitReason    string
		lastExitTimestamp time.Time
		severity          issue.Severity
	}

	var best *candidate

	for _, pod := range pods {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Name != app.Name {
				continue
			}
			if cs.LastTerminationState.Terminated == nil {
				continue
			}

			finishedAt := cs.LastTerminationState.Terminated.FinishedAt.Time
			age := now.Sub(finishedAt)

			var sev issue.Severity
			switch {
			case cs.RestartCount >= restartLoopCriticalMinRestarts && age <= restartLoopCriticalWindow:
				sev = issue.SeverityCritical
			case cs.RestartCount >= restartLoopWarningMinRestarts && age <= restartLoopWarningWindow:
				sev = issue.SeverityWarning
			default:
				continue
			}

			reason := cs.LastTerminationState.Terminated.Reason
			if reason == "" {
				reason = fmt.Sprintf("ExitCode:%d", cs.LastTerminationState.Terminated.ExitCode)
			}

			c := &candidate{
				restartCount:      cs.RestartCount,
				lastExitReason:    reason,
				lastExitTimestamp: finishedAt,
				severity:          sev,
			}

			if best == nil {
				best = c
			} else if c.severity == issue.SeverityCritical && best.severity != issue.SeverityCritical {
				best = c
			} else if c.severity == best.severity && c.restartCount > best.restartCount {
				best = c
			}
		}
	}

	if best == nil {
		return nil
	}

	age := now.Sub(best.lastExitTimestamp)
	minutesAgo := int(age.Minutes())

	var timeDesc string
	switch {
	case minutesAgo < 1:
		timeDesc = "less than a minute ago"
	case minutesAgo == 1:
		timeDesc = "1 minute ago"
	default:
		timeDesc = fmt.Sprintf("%d minutes ago", minutesAgo)
	}

	message := fmt.Sprintf("Application has restarted %d times, most recently %s (%s)", best.restartCount, timeDesc, best.lastExitReason)

	return &Issue{
		IssueType:    issue.IssueTypeApplicationRestartLoop,
		ResourceName: app.Name,
		ResourceType: issue.ResourceTypeApplication,
		Team:         team,
		Env:          env,
		Severity:     best.severity,
		Message:      message,
		IssueDetails: issue.RestartLoopIssueDetails{
			RestartCount:      int(best.restartCount),
			LastExitReason:    best.lastExitReason,
			LastExitTimestamp: best.lastExitTimestamp,
		},
	}
}
