package postgres

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestToPostgresBranch(t *testing.T) {
	createdAt := time.Date(2025, time.December, 31, 12, 0, 0, 0, time.UTC)
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "nais.io/v1", "kind": "PostgresBranch",
		"metadata": map[string]any{"name": nais_io_v1.PostgresBranchObjectName("orders", "restored"), "namespace": "my-team", "creationTimestamp": createdAt.Format(time.RFC3339)},
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
	if !got.CreatedAt.Equal(createdAt) {
		t.Errorf("createdAt = %v, want %v", got.CreatedAt, createdAt)
	}
	if got.GetName() != obj.GetName() {
		t.Errorf("watcher name = %q, want %q", got.GetName(), obj.GetName())
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
		"spec":     map[string]any{"majorVersion": "17", "highAvailability": true, "extensions": []any{map[string]any{"name": "pg_trgm"}, map[string]any{"name": "uuid-ossp"}}, "resources": map[string]any{"cpu": "100m", "memory": "2Gi", "diskSize": "10Gi"}},
		"status":   map[string]any{"activeBranch": "restored"},
	}}
	got, err := toPostgres(obj, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "orders" || got.MajorVersion != "17" || got.ActiveBranch == nil || *got.ActiveBranch != "restored" {
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

func TestPostgresExtensionFromName(t *testing.T) {
	tests := []struct {
		name string
		want PostgresExtension
	}{
		{name: "tablefunc", want: "TABLEFUNC"},
		{name: "citext", want: "CITEXT"},
		{name: "pageinspect", want: "PAGEINSPECT"},
		{name: "hstore", want: "HSTORE"},
		{name: "pg_logicalinspect", want: "PG_LOGICALINSPECT"},
		{name: "xml2", want: "XML2"},
		{name: "file_fdw", want: "FILE_FDW"},
		{name: "pgrowlocks", want: "PGROWLOCKS"},
		{name: "pg_visibility", want: "PG_VISIBILITY"},
		{name: "pgstattuple", want: "PGSTATTUPLE"},
		{name: "dict_int", want: "DICT_INT"},
		{name: "pg_freespacemap", want: "PG_FREESPACEMAP"},
		{name: "tcn", want: "TCN"},
		{name: "bloom", want: "BLOOM"},
		{name: "btree_gin", want: "BTREE_GIN"},
		{name: "unaccent", want: "UNACCENT"},
		{name: "seg", want: "SEG"},
		{name: "tsm_system_rows", want: "TSM_SYSTEM_ROWS"},
		{name: "insert_username", want: "INSERT_USERNAME"},
		{name: "dict_xsyn", want: "DICT_XSYN"},
		{name: "tsm_system_time", want: "TSM_SYSTEM_TIME"},
		{name: "postgres_fdw", want: "POSTGRES_FDW"},
		{name: "autoinc", want: "AUTOINC"},
		{name: "earthdistance", want: "EARTHDISTANCE"},
		{name: "amcheck", want: "AMCHECK"},
		{name: "fuzzystrmatch", want: "FUZZYSTRMATCH"},
		{name: "plpgsql", want: "PLPGSQL"},
		{name: "refint", want: "REFINT"},
		{name: "uuid-ossp", want: "UUID_OSSP"},
		{name: "intarray", want: "INTARRAY"},
		{name: "btree_gist", want: "BTREE_GIST"},
		{name: "pg_buffercache", want: "PG_BUFFERCACHE"},
		{name: "pg_stat_statements", want: "PG_STAT_STATEMENTS"},
		{name: "intagg", want: "INTAGG"},
		{name: "ltree", want: "LTREE"},
		{name: "cube", want: "CUBE"},
		{name: "pg_walinspect", want: "PG_WALINSPECT"},
		{name: "dblink", want: "DBLINK"},
		{name: "pg_prewarm", want: "PG_PREWARM"},
		{name: "sslinfo", want: "SSLINFO"},
		{name: "pgcrypto", want: "PGCRYPTO"},
		{name: "moddatetime", want: "MODDATETIME"},
		{name: "pg_trgm", want: "PG_TRGM"},
		{name: "pg_surgery", want: "PG_SURGERY"},
		{name: "isn", want: "ISN"},
		{name: "lo", want: "LO"},
		{name: "vector", want: "VECTOR"},
		{name: "pgaudit", want: "PGAUDIT"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PostgresExtensionFromName(tt.name)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("PostgresExtensionFromName(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}

	_, err := PostgresExtensionFromName("unknown")
	if err == nil {
		t.Fatal("expected unsupported extension to return an error")
	}

	ctx := context.Background()
	presenter := apierror.GetErrorPresenter(logrus.New())
	presented := presenter(ctx, graphql.DefaultErrorPresenter(ctx, err))
	if !strings.Contains(presented.Message, `"unknown"`) {
		t.Errorf("presented error = %q, want to identify the unsupported extension", presented.Message)
	}

	if _, err := PostgresExtensionFromName("pg_failover_slots"); err == nil {
		t.Fatal("expected extension unavailable in pg_available_extensions to return an error")
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
		{name: "disk below minimum", disk: "9Gi", wantErr: true},
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

func TestPostgresStateFromConditions(t *testing.T) {
	tests := []struct {
		name       string
		reconciled bool
		conditions []metav1.Condition
		want       PostgresBranchState
	}{
		{name: "not reconciled", want: PostgresBranchStateProgressing},
		{name: "unrecoverable recovery failure", conditions: []metav1.Condition{{Type: "RecoveryFailed", Status: metav1.ConditionTrue, Reason: "Unrecoverable"}}, want: PostgresBranchStateDegraded},
		{name: "recovery failure not terminal", conditions: []metav1.Condition{{Type: "RecoveryFailed", Status: metav1.ConditionFalse, Reason: "Recovering"}}, want: PostgresBranchStateProgressing},
		{name: "ready false without recovery failure", reconciled: true, conditions: []metav1.Condition{{Type: "Ready", Status: metav1.ConditionFalse, Reason: "RecoveryFailed"}}, want: PostgresBranchStateProgressing},
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
