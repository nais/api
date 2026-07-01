package issue

import (
	"reflect"
	"testing"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/issue/issuesql"
)

func TestBuildFacets(t *testing.T) {
	rows := []*issuesql.FacetsForIssuesRow{
		{Severity: "CRITICAL", ResourceType: "APPLICATION", Env: "prod", IssueType: "DEPRECATED_INGRESS", FilteredCount: 3},
		{Severity: "CRITICAL", ResourceType: "APPLICATION", Env: "dev", IssueType: "DEPRECATED_INGRESS", FilteredCount: 1},
		{Severity: "WARNING", ResourceType: "JOB", Env: "prod", IssueType: "LAST_RUN_FAILED", FilteredCount: 2},
		{Severity: "TODO", ResourceType: "SQLINSTANCE", Env: "prod", IssueType: "SQLINSTANCE_VERSION", FilteredCount: 0},
	}

	tests := []struct {
		name              string
		rows              []*issuesql.FacetsForIssuesRow
		wantSeverities    []IssueSeverityFacetItem
		wantResourceTypes []IssueResourceTypeFacetItem
		wantEnvironments  []model.StringFacetItem
		wantIssueTypes    []IssueTypeFacetItem
	}{
		{
			name: "mixed rows: seeded values with zero filtered_count are included",
			rows: rows,
			wantSeverities: []IssueSeverityFacetItem{
				{Severity: SeverityCritical, Count: 4},
				{Severity: SeverityTodo, Count: 0},
				{Severity: SeverityWarning, Count: 2},
			},
			wantResourceTypes: []IssueResourceTypeFacetItem{
				{ResourceType: ResourceTypeApplication, Count: 4},
				{ResourceType: ResourceTypeJob, Count: 2},
				{ResourceType: ResourceTypeSQLInstance, Count: 0},
			},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 1},
				{Value: "prod", Count: 5},
			},
			wantIssueTypes: []IssueTypeFacetItem{
				{IssueType: IssueTypeDeprecatedIngress, Count: 4},
				{IssueType: IssueTypeLastRunFailed, Count: 2},
				{IssueType: IssueTypeSqlInstanceVersion, Count: 0},
			},
		},
		{
			name: "with filter: TODO row has filtered_count=0 but is still seeded",
			rows: []*issuesql.FacetsForIssuesRow{
				{Severity: "CRITICAL", ResourceType: "APPLICATION", Env: "prod", IssueType: "DEPRECATED_INGRESS", FilteredCount: 3},
				{Severity: "WARNING", ResourceType: "JOB", Env: "dev", IssueType: "LAST_RUN_FAILED", FilteredCount: 0},
			},
			wantSeverities: []IssueSeverityFacetItem{
				{Severity: SeverityCritical, Count: 3},
				{Severity: SeverityWarning, Count: 0},
			},
			wantResourceTypes: []IssueResourceTypeFacetItem{
				{ResourceType: ResourceTypeApplication, Count: 3},
				{ResourceType: ResourceTypeJob, Count: 0},
			},
			wantEnvironments: []model.StringFacetItem{
				{Value: "dev", Count: 0},
				{Value: "prod", Count: 3},
			},
			wantIssueTypes: []IssueTypeFacetItem{
				{IssueType: IssueTypeDeprecatedIngress, Count: 3},
				{IssueType: IssueTypeLastRunFailed, Count: 0},
			},
		},
		{
			name:              "empty rows returns empty facets",
			rows:              nil,
			wantSeverities:    []IssueSeverityFacetItem{},
			wantResourceTypes: []IssueResourceTypeFacetItem{},
			wantEnvironments:  []model.StringFacetItem{},
			wantIssueTypes:    []IssueTypeFacetItem{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildFacets(tt.rows)

			if !reflect.DeepEqual(got.Severities, tt.wantSeverities) {
				t.Errorf("Severities =\n  %v\nwant\n  %v", got.Severities, tt.wantSeverities)
			}
			if !reflect.DeepEqual(got.ResourceTypes, tt.wantResourceTypes) {
				t.Errorf("ResourceTypes =\n  %v\nwant\n  %v", got.ResourceTypes, tt.wantResourceTypes)
			}
			if !reflect.DeepEqual(got.Environments, tt.wantEnvironments) {
				t.Errorf("Environments =\n  %v\nwant\n  %v", got.Environments, tt.wantEnvironments)
			}
			if !reflect.DeepEqual(got.IssueTypes, tt.wantIssueTypes) {
				t.Errorf("IssueTypes =\n  %v\nwant\n  %v", got.IssueTypes, tt.wantIssueTypes)
			}
		})
	}
}
