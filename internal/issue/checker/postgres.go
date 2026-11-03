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
}

func (p Postgres) Run(_ context.Context) ([]Issue, error) {
	if p.PostgresBranchWatcher == nil || !p.PostgresBranchWatcher.Enabled() {
		return nil, nil
	}

	var issues []Issue
	for _, instance := range p.PostgresBranchWatcher.All() {
		branch := instance.Obj
		if branch.State != postgres.PostgresBranchStateDegraded {
			continue
		}
		issues = append(issues, Issue{
			IssueType:    issue.IssueTypePostgresBranch,
			ResourceName: branch.GetName(),
			ResourceType: issue.ResourceTypePostgresBranch,
			Team:         branch.TeamSlug.String(),
			Env:          branch.EnvironmentName,
			Severity:     issue.SeverityCritical,
			Message:      fmt.Sprintf("Postgres branch %q of %q has reported a failure.", branch.Name, branch.PostgresName),
		})
	}
	return issues, nil
}
