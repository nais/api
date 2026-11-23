package postgres

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/nais/api/internal/validate"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestToPostgresWithoutStatus(t *testing.T) {
	obj := physicalPostgresObject("orders", nil)
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.TeamSlug != "team" || got.EnvironmentName != "dev" {
		t.Fatalf("unexpected Postgres: %+v", got)
	}
	createdAt := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("createdAt = %v, want %v", got.CreatedAt, createdAt)
	}
	if got.GetName() != obj.GetName() {
		t.Error("resource name changed")
	}
	state, config, err := PhysicalStatus(t.Context(), got)
	if err != nil || state != PostgresStateProgressing || config != nil {
		t.Fatalf("status = %v, %v, %v", state, config, err)
	}
}

func TestToPostgres(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": "orders", "namespace": "my-team"},
		"spec":     map[string]any{"majorVersion": "17", "highAvailability": true, "extensions": []any{map[string]any{"name": "pg_trgm"}, map[string]any{"name": "uuid-ossp"}}, "resources": map[string]any{"cpu": "100m", "memory": "2Gi", "diskSize": "10Gi"}},
	}}
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.MajorVersion != "17" {
		t.Errorf("unexpected Postgres: %+v", got)
	}
	if !reflect.DeepEqual(got.Extensions, []string{"pg_trgm", "uuid-ossp"}) {
		t.Errorf("extensions = %v, want [pg_trgm uuid-ossp]", got.Extensions)
	}
	if got.Resources.CPU == nil || *got.Resources.CPU != "100m" || got.Resources.Memory == nil || *got.Resources.Memory != "2Gi" || got.Resources.DiskSize == nil || *got.Resources.DiskSize != "10Gi" {
		t.Errorf("unexpected Postgres resources: %+v", got.Resources)
	}
}

func TestToPostgresWithoutExtensions(t *testing.T) {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": "orders", "namespace": "my-team"},
		"spec":     map[string]any{"majorVersion": "17"},
	}}
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Extensions == nil || len(got.Extensions) != 0 {
		t.Errorf("extensions = %v, want an empty list", got.Extensions)
	}
}

func TestToPostgresPreservesExtensionNames(t *testing.T) {
	names := []string{"pgaudit", "uuid-ossp", "postgis", "timescaledb", "future_extension"}
	extensions := make([]any, 0, len(names))
	for _, name := range names {
		extensions = append(extensions, map[string]any{"name": name})
	}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "Postgres",
		"metadata": map[string]any{"name": "orders", "namespace": "my-team"},
		"spec":     map[string]any{"majorVersion": "18", "extensions": extensions},
	}}
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Extensions, names) {
		t.Errorf("extensions = %v, want %v", got.Extensions, names)
	}
}

func TestValidateQuantities(t *testing.T) {
	tests := []struct {
		name    string
		cpu     string
		memory  string
		disk    string
		wantErr bool
	}{
		{name: "inclusive limits", cpu: "8000m", memory: "32768Mi", disk: "1000Gi"},
		{name: "cpu above maximum", cpu: "8.001", wantErr: true},
		{name: "memory above maximum", memory: "33Gi", wantErr: true},
		{name: "disk below minimum", disk: "4Gi", wantErr: true},
		{name: "disk above maximum", disk: "1001Gi", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verr := validate.New()
			cpu, memory, disk := tt.cpu, tt.memory, tt.disk
			var cpuValue, memoryValue, diskValue *string
			if cpu != "" {
				cpuValue = &cpu
			}
			if memory != "" {
				memoryValue = &memory
			}
			if disk != "" {
				diskValue = &disk
			}
			validateQuantities(verr, cpuValue, memoryValue, diskValue)
			err := verr.NilIfEmpty()
			if (err != nil) != tt.wantErr {
				t.Errorf("validation error = %v, want error %t", err, tt.wantErr)
			}
		})
	}
}

func TestPostgresInputDiskSizeValidation(t *testing.T) {
	for _, tt := range []struct {
		diskSize string
		wantErr  string
	}{
		{diskSize: "4Gi", wantErr: `diskSize: "4Gi" must be at least 5Gi.`},
		{diskSize: "5119Mi", wantErr: `diskSize: "5119Mi" must be at least 5Gi.`},
		{diskSize: "5Gi"},
		{diskSize: "9Gi"},
	} {
		t.Run(tt.diskSize, func(t *testing.T) {
			create := CreatePostgresInput{
				Name: "orders", EnvironmentName: "dev", TeamSlug: "team-a", MajorVersion: "18", DiskSize: &tt.diskSize,
			}
			update := UpdatePostgresInput{
				Name: "orders", EnvironmentName: "dev", TeamSlug: "team-a", DiskSize: &tt.diskSize,
			}
			for name, validationErrors := range map[string]*validate.ValidationErrors{
				"create": create.ValidationErrors(context.Background()),
				"update": update.ValidationErrors(context.Background()),
			} {
				t.Run(name, func(t *testing.T) {
					if got := validationErrors.Error(); got != tt.wantErr {
						t.Errorf("validation error = %q, want %q", got, tt.wantErr)
					}
				})
			}
		})
	}
}

func TestPostgresStateFromConditions(t *testing.T) {
	tests := []struct {
		name       string
		reconciled bool
		conditions []metav1.Condition
		want       PostgresState
	}{
		{name: "not reconciled", want: PostgresStateProgressing},
		{name: "unrecoverable recovery failure", conditions: []metav1.Condition{{Type: "RecoveryFailed", Status: metav1.ConditionTrue, Reason: "Unrecoverable"}}, want: PostgresStateDegraded},
		{name: "recovery failure not terminal", conditions: []metav1.Condition{{Type: "RecoveryFailed", Status: metav1.ConditionFalse, Reason: "Recovering"}}, want: PostgresStateProgressing},
		{name: "ready false without recovery failure", reconciled: true, conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse, Reason: "RecoveryFailed"}}, want: PostgresStateProgressing},
		{name: "healthy", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster in healthy state"}}, want: PostgresStateAvailable},
		{name: "still starting", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionFalse, Message: "Cluster is in phase: "}}, want: PostgresStateProgressing},
		{name: "unrecoverable", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster is unrecoverable and needs manual intervention"}}, want: PostgresStateDegraded},
		{name: "plugin failure", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Cluster cannot proceed to reconciliation due to an error while interacting with plugins"}}, want: PostgresStateDegraded},
		{name: "other phase", reconciled: true, conditions: []metav1.Condition{{Type: "cluster.postgresql.cnpg.io/ObservedState", Status: metav1.ConditionTrue, Message: "Cluster is in phase: Online upgrade in progress"}}, want: PostgresStateProgressing},
		{name: "missing", reconciled: true, want: PostgresStateProgressing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := postgresStateFromConditions(tt.conditions, tt.reconciled); got != tt.want {
				t.Errorf("state = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestToPostgresBackupRetentionDays(t *testing.T) {
	for name, tt := range map[string]struct {
		spec map[string]any
		want int
	}{
		"default":  {spec: map[string]any{"majorVersion": "17"}, want: int(nais_io_v1.DefaultBackupRetentionDays)},
		"explicit": {spec: map[string]any{"majorVersion": "17", "backup": map[string]any{"retentionDays": int64(21)}}, want: 21},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := toPostgres(&unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "nais.io/v1", "kind": "Postgres",
				"metadata": map[string]any{"name": "orders", "namespace": "my-team"},
				"spec":     tt.spec,
			}}, "dev")
			if err != nil {
				t.Fatal(err)
			}
			if got.BackupRetentionDays != tt.want {
				t.Errorf("BackupRetentionDays = %d, want %d", got.BackupRetentionDays, tt.want)
			}
		})
	}
}
