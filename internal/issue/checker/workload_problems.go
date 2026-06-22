package checker

import (
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/issue"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

// workloadProblems surfaces every problem reported by naiserator in a workload's
// `.status.problems[]`. This is the canonical, user-facing list of issues naiserator
// wants to expose through the API.
func (w Workload) workloadProblems(wl NaisWorkload, env string, resourceType issue.ResourceType) []*Issue {
	status := wl.GetStatus()
	if status == nil || status.Problems == nil {
		return nil
	}

	var ret []*Issue
	for _, problem := range *status.Problems {
		ret = append(ret, &Issue{
			IssueType:    issue.IssueTypeWorkloadProblem,
			ResourceName: wl.GetName(),
			ResourceType: resourceType,
			Team:         wl.GetNamespace(),
			Env:          env,
			Severity:     problemSeverity(problem.Type),
			Message:      problem.Message,
			IssueDetails: issue.WorkloadProblemIssueDetails{
				ProblemType: problemType(problem.Type),
				Source:      problem.Source,
				EndOfLife:   problemEndOfLife(problem.EndOfLife),
			},
		})
	}

	return ret
}

func problemEndOfLife(eol *string) *scalar.Date {
	if eol == nil || *eol == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, *eol)
	if err != nil {
		return nil
	}
	d := scalar.NewDate(t)
	return &d
}

func problemSeverity(kind nais_io_v1.ProblemKind) issue.Severity {
	switch kind {
	case nais_io_v1.ProblemKindError:
		return issue.SeverityCritical
	case nais_io_v1.ProblemKindWarning:
		return issue.SeverityWarning
	case nais_io_v1.ProblemKindDeprecation:
		return issue.SeverityTodo
	default:
		return issue.SeverityWarning
	}
}

func problemType(kind nais_io_v1.ProblemKind) issue.WorkloadProblemType {
	switch kind {
	case nais_io_v1.ProblemKindError:
		return issue.WorkloadProblemTypeError
	case nais_io_v1.ProblemKindWarning:
		return issue.WorkloadProblemTypeWarning
	case nais_io_v1.ProblemKindDeprecation:
		return issue.WorkloadProblemTypeDeprecation
	default:
		return issue.WorkloadProblemTypeWarning
	}
}
