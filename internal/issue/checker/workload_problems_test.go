package checker

import (
	"testing"

	"github.com/nais/api/internal/issue"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
)

func appWithProblems(name string, problems ...nais_io_v1.Problem) *nais_io_v1alpha1.Application {
	app := makeApp(name)
	if problems != nil {
		p := append([]nais_io_v1.Problem(nil), problems...)
		app.Status.Problems = &p
	}
	return app
}

func TestWorkloadProblems(t *testing.T) {
	w := Workload{log: logrus.New()}

	t.Run("no status problems returns no issues", func(t *testing.T) {
		got := w.workloadProblems(makeApp("myapp"), "dev", issue.ResourceTypeApplication)
		if got != nil {
			t.Fatalf("expected no issues, got %d", len(got))
		}
	})

	t.Run("FailedPrepare error is surfaced as a critical issue", func(t *testing.T) {
		// Mirrors naiserator's FailedPrepare behaviour (e.g. galning): the
		// synchronization condition is transient, but the underlying problem
		// is recorded as an Error in .status.problems[].
		app := appWithProblems("galning", nais_io_v1.Problem{
			Type:    nais_io_v1.ProblemKindError,
			Message: "get wanted image: the-g-team/galning: external image resource not found",
		})

		got := w.workloadProblems(app, "prod", issue.ResourceTypeApplication)
		if len(got) != 1 {
			t.Fatalf("expected 1 issue, got %d", len(got))
		}
		iss := got[0]
		if iss.IssueType != issue.IssueTypeWorkloadProblem {
			t.Errorf("expected issue type %s, got %s", issue.IssueTypeWorkloadProblem, iss.IssueType)
		}
		if iss.Severity != issue.SeverityCritical {
			t.Errorf("expected severity %s, got %s", issue.SeverityCritical, iss.Severity)
		}
		if iss.Message != "get wanted image: the-g-team/galning: external image resource not found" {
			t.Errorf("unexpected message: %s", iss.Message)
		}
		details, ok := iss.IssueDetails.(issue.WorkloadProblemIssueDetails)
		if !ok {
			t.Fatalf("expected WorkloadProblemIssueDetails, got %T", iss.IssueDetails)
		}
		if details.ProblemType != issue.WorkloadProblemTypeError {
			t.Errorf("expected problem type %s, got %s", issue.WorkloadProblemTypeError, details.ProblemType)
		}
	})

	t.Run("maps every problem kind to a severity and preserves details", func(t *testing.T) {
		app := appWithProblems("multi",
			nais_io_v1.Problem{Type: nais_io_v1.ProblemKindError, Message: "boom", Source: new(".spec.image")},
			nais_io_v1.Problem{Type: nais_io_v1.ProblemKindWarning, Message: "watch out"},
			nais_io_v1.Problem{Type: nais_io_v1.ProblemKindDeprecation, Message: "going away", EndOfLife: new("2026-01-01")},
		)

		got := w.workloadProblems(app, "dev", issue.ResourceTypeApplication)
		if len(got) != 3 {
			t.Fatalf("expected 3 issues, got %d", len(got))
		}

		wantSeverity := []issue.Severity{issue.SeverityCritical, issue.SeverityWarning, issue.SeverityTodo}
		wantType := []issue.WorkloadProblemType{issue.WorkloadProblemTypeError, issue.WorkloadProblemTypeWarning, issue.WorkloadProblemTypeDeprecation}
		for i, iss := range got {
			if iss.Severity != wantSeverity[i] {
				t.Errorf("issue %d: expected severity %s, got %s", i, wantSeverity[i], iss.Severity)
			}
			details, ok := iss.IssueDetails.(issue.WorkloadProblemIssueDetails)
			if !ok {
				t.Fatalf("issue %d: expected WorkloadProblemIssueDetails, got %T", i, iss.IssueDetails)
			}
			if details.ProblemType != wantType[i] {
				t.Errorf("issue %d: expected problem type %s, got %s", i, wantType[i], details.ProblemType)
			}
		}

		if src := got[0].IssueDetails.(issue.WorkloadProblemIssueDetails).Source; src == nil || *src != ".spec.image" {
			t.Errorf("expected source .spec.image to be preserved, got %v", src)
		}
		if eol := got[2].IssueDetails.(issue.WorkloadProblemIssueDetails).EndOfLife; eol == nil || eol.String() != "2026-01-01" {
			t.Errorf("expected endOfLife to be preserved, got %v", eol)
		}
	})
}
