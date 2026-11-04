package graph

import (
	"testing"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/vektah/gqlparser/v2"
)

func TestTenantAndScopedFilters(t *testing.T) {
	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
	for _, tt := range []struct {
		name      string
		query     string
		wantError bool
	}{
		{
			name: "tenant filter variables",
			query: `query($activity: TenantActivityLogFilter, $issues: TenantIssueFilter) {
				activityLog(filter: $activity) { pageInfo { totalCount } }
				issues(filter: $issues) { pageInfo { totalCount } }
			}`,
		},
		{
			name: "existing team filter variables",
			query: `query($activity: ActivityLogFilter, $issues: IssueFilter) {
				team(slug: "team-a") {
					activityLog(filter: $activity) { pageInfo { totalCount } }
					issues(filter: $issues) { pageInfo { totalCount } }
				}
			}`,
		},
		{
			name: "activity logger interface keeps scoped filter",
			query: `query($filter: ActivityLogFilter) {
				node(id: "resource-id") {
					... on ActivityLogger { activityLog(filter: $filter) { pageInfo { totalCount } } }
				}
			}`,
		},
		{
			name:      "team activity rejects team selection",
			query:     `{ team(slug: "team-a") { activityLog(filter: {teamScope: {teamSlugs: ["other"]}}) { __typename } } }`,
			wantError: true,
		},
		{
			name:      "team issues reject team selection",
			query:     `{ team(slug: "team-a") { issues(filter: {teamScope: {teamSlugs: ["other"]}}) { __typename } } }`,
			wantError: true,
		},
		{
			name:      "old root filter variable requires migration",
			query:     `query($filter: ActivityLogFilter) { activityLog(filter: $filter) { __typename } }`,
			wantError: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, errors := gqlparser.LoadQuery(schema, tt.query)
			if (len(errors) > 0) != tt.wantError {
				t.Errorf("query validation errors = %v, wantError = %v", errors, tt.wantError)
			}
		})
	}
}
