package checker

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/persistence/postgres"
)

type Postgres struct {
	PostgresWatcher *watcher.Watcher[*postgres.Postgres]
}

func (p Postgres) Run(ctx context.Context) ([]Issue, error) {
	var postgreses []*postgres.Postgres
	if p.PostgresWatcher != nil && p.PostgresWatcher.Enabled() {
		postgreses = watcher.Objects(p.PostgresWatcher.All())
	}

	var issues []Issue
	for _, pg := range postgreses {
		state, _, err := postgres.PhysicalStatus(ctx, pg)
		if err != nil {
			return nil, err
		}
		if state != postgres.PostgresBranchStateDegraded {
			continue
		}
		issues = append(issues, Issue{
			IssueType:    issue.IssueTypePostgres,
			ResourceName: pg.Name,
			ResourceType: issue.ResourceTypePostgres,
			Team:         pg.TeamSlug.String(),
			Env:          pg.EnvironmentName,
			Severity:     issue.SeverityCritical,
			Message:      fmt.Sprintf("Postgres %q has reported a failure.", pg.Name),
		})
	}
	return issues, nil
}
