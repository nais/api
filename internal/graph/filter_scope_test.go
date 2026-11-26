package graph

import (
	"os"
	"strings"
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

func TestV0sMigrationQueryExamples(t *testing.T) {
	document, err := os.ReadFile("../../docs/v0s_console_migration.md")
	if err != nil {
		t.Fatal(err)
	}
	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
	blocks := strings.Split(string(document), "```graphql\n")[1:]
	if len(blocks) == 0 {
		t.Fatal("migration contract contains no GraphQL examples")
	}
	for _, block := range blocks {
		query, _, ok := strings.Cut(block, "```")
		if !ok {
			t.Fatal("unterminated GraphQL example")
		}
		if _, errors := gqlparser.LoadQuery(schema, query); len(errors) > 0 {
			t.Errorf("invalid migration example:\n%s\n%v", query, errors)
		}
	}
}

func TestV0sContextFilters(t *testing.T) {
	schema := gengql.NewExecutableSchema(gengql.Config{}).Schema()
	for _, tt := range []struct {
		name      string
		query     string
		wantError bool
	}{
		{
			name: "context-specific filter variables",
			query: `query(
				$global: VulnerabilityFindingsFilter
				$team: TeamVulnerabilityFindingsFilter
				$environment: TeamEnvironmentVulnerabilityFindingsFilter
				$findings: WorkloadVulnerabilityFindingsFilter
				$overview: WorkloadVulnerabilityOverviewFilter
				$suppressions: VulnerabilitySuppressionFilter
				$teamSuppressions: TeamVulnerabilitySuppressionsFilter
				$environmentSuppressions: TeamEnvironmentVulnerabilitySuppressionsFilter
			) {
				vulnerabilityFindings(filter: $global) { __typename }
				vulnerabilitySuppressions(filter: $suppressions) { __typename }
				team(slug: "team-a") {
					vulnerabilityFindings(filter: $team) { __typename }
					vulnerabilitySuppressions(filter: $teamSuppressions) { __typename }
					environment(name: "dev") {
						vulnerabilityFindings(filter: $environment) { __typename }
						vulnerabilitySuppressions(filter: $environmentSuppressions) { __typename }
						workload(name: "app") {
							vulnerabilityFindings(filter: $findings) { __typename }
							vulnerabilityOverview(filter: $overview) { __typename }
							... on Application { vulnerabilityFindings(filter: $findings) { __typename } }
							... on Job { vulnerabilityOverview(filter: $overview) { __typename } }
						}
					}
				}
			}`,
		},
		{
			name: "combined workload and finding criteria",
			query: `{
				vulnerabilityFindings(filter: {
					teamScope: {memberOfViewer: true}, environments: ["dev"],
					labels: [{key: "owner"}], packageType: "npm", suppressed: INCLUDE
				}) { summary { findingCount } }
				team(slug: "team-a") {
					vulnerabilityFindings(filter: {
						environments: ["dev"], labels: [{key: "owner"}],
						vulnerabilityIdentifiers: ["CVE-1"], onlyWithFix: true
					}) { affectedWorkloads { pageInfo { totalCount } } }
				}
			}`,
		},
		{
			name:      "global findings filter cannot be reused in team context",
			query:     `query($filter: VulnerabilityFindingsFilter) { team(slug: "team-a") { vulnerabilityFindings(filter: $filter) { __typename } } }`,
			wantError: true,
		},
		{
			name:      "team findings filter cannot be reused in environment context",
			query:     `query($filter: TeamVulnerabilityFindingsFilter) { team(slug: "team-a") { environment(name: "dev") { vulnerabilityFindings(filter: $filter) { __typename } } } }`,
			wantError: true,
		},
		{
			name:      "workload findings filter cannot be reused for overview",
			query:     `query($filter: WorkloadVulnerabilityFindingsFilter) { team(slug: "team-a") { environment(name: "dev") { workload(name: "app") { vulnerabilityOverview(filter: $filter) { __typename } } } } }`,
			wantError: true,
		},
		{
			name:      "global suppression filter cannot be reused in team context",
			query:     `query($filter: VulnerabilitySuppressionFilter) { team(slug: "team-a") { vulnerabilitySuppressions(filter: $filter) { __typename } } }`,
			wantError: true,
		},
		{
			name:      "team suppression filter cannot be reused in environment context",
			query:     `query($filter: TeamVulnerabilitySuppressionsFilter) { team(slug: "team-a") { environment(name: "dev") { vulnerabilitySuppressions(filter: $filter) { __typename } } } }`,
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
