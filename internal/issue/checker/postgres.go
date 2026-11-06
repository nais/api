package checker

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/postgres"
)

type Postgres struct {
	PostgresBranchWatcher *watcher.Watcher[*postgres.PostgresBranch]
	PostgresWatcher       *watcher.Watcher[*postgres.Postgres]
}

func (p Postgres) Run(_ context.Context) ([]Issue, error) {
	var branches []*postgres.PostgresBranch
	var postgreses []*postgres.Postgres
	if p.PostgresBranchWatcher != nil && p.PostgresBranchWatcher.Enabled() {
		branches = watcher.Objects(p.PostgresBranchWatcher.All())
	}
	if p.PostgresWatcher != nil && p.PostgresWatcher.Enabled() {
		postgreses = watcher.Objects(p.PostgresWatcher.All())
	}

	var issues []Issue
	for _, branch := range postgres.BranchViews(postgreses, branches) {
		if branch.State != postgres.PostgresBranchStateDegraded {
			continue
		}
		message := fmt.Sprintf("Postgres branch %q of %q has reported a failure.", branch.Name, branch.PostgresName)
		if branch.ClusterName != "" {
			message = fmt.Sprintf("Postgres %q has reported a failure.", branch.PostgresName)
		}
		// Keep the legacy issue identity until issue/schema contraction; its
		// resource resolves through the virtual main after branch detachment.
		issues = append(issues, Issue{
			IssueType:    issue.IssueTypePostgresBranch,
			ResourceName: branch.GetName(),
			ResourceType: issue.ResourceTypePostgresBranch,
			Team:         branch.TeamSlug.String(),
			Env:          branch.EnvironmentName,
			Severity:     issue.SeverityCritical,
			Message:      message,
		})
	}
	return issues, nil
}
