package postgres

import (
	"context"
	"reflect"
	"testing"

	"github.com/nais/api/internal/slug"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestToPostgresInstance(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresInstance",
		"metadata": map[string]any{"name": "orders-restored", "namespace": "my-team"},
		"spec":     map[string]any{"postgres": "orders"},
		"status":   map[string]any{"reconcilePhase": "Completed", "conditions": []any{map[string]any{"type": "cluster.postgresql.cnpg.io/ObservedState", "status": "True", "lastTransitionTime": "2026-01-01T00:00:00Z", "reason": "Reconciled", "message": "Cluster is in phase: Cluster in healthy state"}}},
	}}
	got, err := toPostgresInstance(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders-restored" || got.PostgresName != "orders" || got.State != PostgresInstanceStateAvailable {
		t.Errorf("unexpected physical instance: %+v", got)
	}
}

func TestToLogicalPostgres(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": "orders", "namespace": "my-team"},
		"spec":     map[string]any{"majorVersion": "17", "highAvailability": true, "resources": map[string]any{"cpu": "100m", "memory": "2Gi", "diskSize": "10Gi"}},
		"status":   map[string]any{"activeInstance": "orders-restored"},
	}}
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.MajorVersion != "17" || got.ActiveInstance == nil || *got.ActiveInstance != "orders-restored" {
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
		want       PostgresInstanceState
	}{
		{name: "not reconciled", want: PostgresInstanceStateProgressing},
		{name: "healthy", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster in healthy state"}}, want: PostgresInstanceStateAvailable},
		{name: "still starting", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionFalse, Message: "Cluster is in phase: "}}, want: PostgresInstanceStateProgressing},
		{name: "unrecoverable", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"}}, want: PostgresInstanceStateDegraded},
		{name: "plugin failure", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster cannot proceed to reconciliation due to an error while interacting with plugins"}}, want: PostgresInstanceStateDegraded},
		{name: "other phase", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Online upgrade in progress"}}, want: PostgresInstanceStateProgressing},
		{name: "missing", reconciled: true, want: PostgresInstanceStateProgressing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postgresStateFromConditions(tt.conditions, tt.reconciled); got != tt.want {
				t.Errorf("state = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestDeletePostgresInput_ValidationErrors(t *testing.T) {
	tests := []struct {
		name          string
		input         DeletePostgresInput
		wantErrFields []string
	}{
		{
			name: "all fields valid",
			input: DeletePostgresInput{
				Name:            "my-db",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: nil,
		},
		{
			name: "empty name",
			input: DeletePostgresInput{
				Name:            "",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: []string{"name"},
		},
		{
			name: "empty environmentName",
			input: DeletePostgresInput{
				Name:            "my-db",
				EnvironmentName: "",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: []string{"environmentName"},
		},
		{
			name: "empty teamSlug",
			input: DeletePostgresInput{
				Name:            "my-db",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug(""),
			},
			wantErrFields: []string{"teamSlug"},
		},
		{
			name: "all fields empty",
			input: DeletePostgresInput{
				Name:            "",
				EnvironmentName: "",
				TeamSlug:        slug.Slug(""),
			},
			wantErrFields: []string{"name", "environmentName", "teamSlug"},
		},
		{
			name: "whitespace-only name treated as empty",
			input: DeletePostgresInput{
				Name:            "   ",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErrFields: []string{"name"},
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
