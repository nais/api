package issue

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/api/internal/issue/issuesql"
)

func TestPostgresIssueConversion(t *testing.T) {
	id := uuid.New()
	for _, tt := range []struct {
		name, issueType, resourceType, resourceName, legacyName string
	}{
		{name: "logical Postgres", issueType: "POSTGRES", resourceType: "POSTGRES", resourceName: "orders"},
		{name: "row from previous release", issueType: "POSTGRES_BRANCH", resourceType: "POSTGRES_BRANCH", resourceName: "orders-main-12345678", legacyName: "orders-main-12345678"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convert(&issuesql.Issue{ID: id, IssueType: tt.issueType, ResourceType: tt.resourceType, ResourceName: tt.resourceName, Team: "team", Env: "dev", Severity: "CRITICAL", Message: "database failure"})
			if err != nil {
				t.Fatal(err)
			}
			pg, ok := got.(*PostgresIssue)
			if !ok {
				t.Fatalf("got %T, want genuine PostgresIssue", got)
			}
			want := &PostgresIssue{Base: Base{ID: newIdent(id.String()), ResourceType: ResourceTypePostgres, ResourceName: tt.resourceName, TeamSlug: "team", EnvironmentName: "dev", Severity: SeverityCritical, Message: "database failure"}, LegacyPhysicalName: tt.legacyName}
			if diff := cmp.Diff(want, pg); diff != "" {
				t.Errorf("converted issue (-want +got):\n%s", diff)
			}
		})
	}
}
