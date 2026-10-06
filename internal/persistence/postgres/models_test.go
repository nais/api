package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestToPostgresBranch(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "restored"), "namespace": "my-team"},
		"spec":     map[string]any{"postgres": "orders", "branchName": "restored"},
		"status":   map[string]any{"reconcilePhase": "Completed", "conditions": []any{map[string]any{"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True", "lastTransitionTime": "2026-01-01T00:00:00Z", "reason": "Reconciled", "message": "Cluster is in phase: Cluster in healthy state"}}},
	}}
	got, err := toPostgresBranch(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "restored" || got.PostgresName != "orders" || got.State != PostgresBranchStateAvailable {
		t.Errorf("unexpected PostgresBranch: %+v", got)
	}
	if got.GetName() != obj.GetName() {
		t.Errorf("watcher name = %q, want %q", got.GetName(), obj.GetName())
	}
	if got.SearchName() != "orders/restored" {
		t.Errorf("search name = %q, want orders/restored", got.SearchName())
	}
	if got.ID().Type != "PBR" {
		t.Errorf("PostgresBranch ID type = %q, want PBR", got.ID().Type)
	}
	team, env, pg, branch, err := parsePostgresBranchIdent(got.ID())
	if err != nil || team != "my-team" || env != "dev" || pg != "orders" || branch != "restored" {
		t.Errorf("branch ID = (%q, %q, %q, %q, %v)", team, env, pg, branch, err)
	}
}

func TestToPostgresBranchWithoutStatus(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "main"), "namespace": "my-team"},
		"spec":     map[string]any{"postgres": "orders", "branchName": "main"},
	}}
	got, err := toPostgresBranch(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.State != PostgresBranchStateProgressing {
		t.Errorf("branch state = %q, want PROGRESSING before status is observed", got.State)
	}
}

func TestToPostgresBranchRejectsMismatchedObjectName(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "main")},
		"spec":     map[string]any{"postgres": "orders", "branchName": "other"},
	}}
	if _, err := toPostgresBranch(obj, "dev"); err == nil {
		t.Fatal("expected a mismatched branch object name to be rejected")
	}
}

func TestToPostgres(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": "orders", "namespace": "my-team"},
		"spec":     map[string]any{"majorVersion": "17", "highAvailability": true, "resources": map[string]any{"cpu": "100m", "memory": "2Gi", "diskSize": "10Gi"}},
		"status":   map[string]any{"activeBranch": "restored"},
	}}
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.MajorVersion != "17" || got.ActiveBranch == nil || *got.ActiveBranch != "restored" {
		t.Errorf("unexpected Postgres: %+v", got)
	}
	if got.Resources.CPU == nil || *got.Resources.CPU != "100m" || got.Resources.Memory == nil || *got.Resources.Memory != "2Gi" || got.Resources.DiskSize == nil || *got.Resources.DiskSize != "10Gi" {
		t.Errorf("unexpected Postgres resources: %+v", got.Resources)
	}
}

func TestPostgresStateFromConditions(t *testing.T) {
	tests := []struct {
		name       string
		reconciled bool
		conditions []metav1.Condition
		want       PostgresBranchState
	}{
		{name: "not reconciled", want: PostgresBranchStateProgressing},
		{name: "healthy", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster in healthy state"}}, want: PostgresBranchStateAvailable},
		{name: "still starting", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionFalse, Message: "Cluster is in phase: "}}, want: PostgresBranchStateProgressing},
		{name: "unrecoverable", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"}}, want: PostgresBranchStateDegraded},
		{name: "plugin failure", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster cannot proceed to reconciliation due to an error while interacting with plugins"}}, want: PostgresBranchStateDegraded},
		{name: "other phase", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Online upgrade in progress"}}, want: PostgresBranchStateProgressing},
		{name: "missing", reconciled: true, want: PostgresBranchStateProgressing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postgresStateFromConditions(tt.conditions, tt.reconciled); got != tt.want {
				t.Errorf("state = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDeletePostgresBranchInput_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         DeletePostgresBranchInput
		wantErrFields []string
	}{
		{
			name: "all fields valid",
			input: DeletePostgresBranchInput{
				Postgres: "my-db", Branch: "main",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: nil,
		},
		{
			name: "empty name",
			input: DeletePostgresBranchInput{
				Postgres: "my-db", Branch: "",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: []string{"branch"},
		},
		{
			name: "empty environmentName",
			input: DeletePostgresBranchInput{
				Postgres: "my-db", Branch: "main",
				EnvironmentName: "",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: []string{"environmentName"},
		},
		{
			name: "empty teamSlug",
			input: DeletePostgresBranchInput{
				Postgres: "my-db", Branch: "main",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug(""),
			},
			wantErrFields: []string{"teamSlug"},
		},
		{
			name: "all fields empty",
			input: DeletePostgresBranchInput{
				Postgres: "", Branch: "",
				EnvironmentName: "",
				TeamSlug:        slug.Slug(""),
			},
			wantErrFields: []string{"postgres", "branch", "environmentName", "teamSlug"},
		},
		{
			name: "whitespace-only name treated as empty",
			input: DeletePostgresBranchInput{
				Postgres: "my-db", Branch: "   ",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: []string{"branch"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verr := tt.input.ValidationErrors(context.Background())
			var gotFields []string
			for _, e := range verr.Errors {
				if e.GraphQLField != nil {
					gotFields = append(gotFields, *e.GraphQLField)
				}
			}
			if !reflect.DeepEqual(gotFields, tt.wantErrFields) {
				t.Errorf("ValidationErrors() fields = %v, want %v", gotFields, tt.wantErrFields)
			}
		})
	}
}
