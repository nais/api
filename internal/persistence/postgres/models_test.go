package postgres

import (
	"testing"

	data_nais_io_v1 "github.com/nais/pgrator/pkg/api/datav1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestPostgresStateFromConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       PostgresInstanceState
	}{
		{
			name: "degraded when degraded is true",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeAvailable, Status: metav1.ConditionTrue},
				{Type: postgresConditionTypeDegraded, Status: metav1.ConditionTrue},
			},
			want: PostgresInstanceStateDegraded,
		},
		{
			name: "progressing when progressing is true",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeProgressing, Status: metav1.ConditionTrue},
			},
			want: PostgresInstanceStateProgressing,
		},
		{
			name: "available when available is true",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeAvailable, Status: metav1.ConditionTrue},
			},
			want: PostgresInstanceStateAvailable,
		},
		{
			name: "available when no recognized true condition",
			conditions: []metav1.Condition{
				{Type: postgresConditionTypeAvailable, Status: metav1.ConditionFalse},
				{Type: postgresConditionTypeProgressing, Status: metav1.ConditionFalse},
			},
			want: PostgresInstanceStateAvailable,
		},
		{
			name: "available when no conditions",
			want: PostgresInstanceStateAvailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := postgresStateFromConditions(tt.conditions)
			if got != tt.want {
				t.Errorf("postgresStateFromConditions() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestToPostgres_MaintenanceWindow(t *testing.T) {
	intPtr := func(v int) *int { return &v }

	tests := []struct {
		name        string
		maintenance *data_nais_io_v1.Maintenance
		wantNil     bool
		wantDay     int
		wantHour    int
	}{
		{
			name: "populates day and hour when maintenance window is present",
			maintenance: &data_nais_io_v1.Maintenance{
				Day:  2,
				Hour: intPtr(5),
			},
			wantDay:  2,
			wantHour: 5,
		},
		{
			name: "defaults hour to 0 when maintenance window hour is omitted",
			maintenance: &data_nais_io_v1.Maintenance{
				Day: 6,
			},
			wantDay:  6,
			wantHour: 0,
		},
		{
			name:    "returns nil maintenance window when not configured",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := newPostgresTestObject(tt.maintenance, nil)
			got := toPostgresFromCRD(t, obj)

			if tt.wantNil {
				if got.MaintenanceWindow != nil {
					t.Fatalf("MaintenanceWindow = %#v, want nil", got.MaintenanceWindow)
				}
				return
			}

			if got.MaintenanceWindow == nil {
				t.Fatalf("MaintenanceWindow = nil, want non-nil")
			}

			if got.MaintenanceWindow.Day != tt.wantDay {
				t.Errorf("MaintenanceWindow.Day = %d, want %d", got.MaintenanceWindow.Day, tt.wantDay)
			}

			if got.MaintenanceWindow.Hour != tt.wantHour {
				t.Errorf("MaintenanceWindow.Hour = %d, want %d", got.MaintenanceWindow.Hour, tt.wantHour)
			}
		})
	}
}

func TestToPostgres_MaintenanceWindow_WithConditions(t *testing.T) {
	hour := 7
	obj := newPostgresTestObject(&data_nais_io_v1.Maintenance{
		Day:  3,
		Hour: &hour,
	}, []metav1.Condition{
		{Type: postgresConditionTypeAvailable, Status: metav1.ConditionTrue},
		{Type: postgresConditionTypeDegraded, Status: metav1.ConditionTrue},
	})

	got := toPostgresFromCRD(t, obj)

	if got.MaintenanceWindow == nil {
		t.Fatalf("MaintenanceWindow = nil, want non-nil")
	}

	if got.MaintenanceWindow.Day != 3 {
		t.Errorf("MaintenanceWindow.Day = %d, want %d", got.MaintenanceWindow.Day, 3)
	}

	if got.MaintenanceWindow.Hour != 7 {
		t.Errorf("MaintenanceWindow.Hour = %d, want %d", got.MaintenanceWindow.Hour, 7)
	}

	if got.State != PostgresInstanceStateDegraded {
		t.Errorf("State = %q, want %q", got.State, PostgresInstanceStateDegraded)
	}
}

func newPostgresTestObject(maintenance *data_nais_io_v1.Maintenance, conditions []metav1.Condition) *data_nais_io_v1.Postgres {
	obj := &data_nais_io_v1.Postgres{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-db",
			Namespace: "my-team",
		},
		Spec: data_nais_io_v1.PostgresSpec{
			Cluster: data_nais_io_v1.PostgresCluster{
				Resources: data_nais_io_v1.PostgresResources{
					DiskSize: resource.MustParse("10Gi"),
					Cpu:      resource.MustParse("100m"),
					Memory:   resource.MustParse("1Gi"),
				},
				MajorVersion: "17",
			},
			MaintenanceWindow: maintenance,
		},
	}

	if len(conditions) > 0 {
		obj.Status = &data_nais_io_v1.PostgresStatus{}
		obj.Status.Conditions = conditions
	}

	return obj
}

func toPostgresFromCRD(t *testing.T, obj *data_nais_io_v1.Postgres) *PostgresInstance {
	t.Helper()

	uMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		t.Fatalf("ToUnstructured() error = %v", err)
	}

	got, err := toPostgres(&unstructured.Unstructured{Object: uMap}, "dev")
	if err != nil {
		t.Fatalf("toPostgres() error = %v", err)
	}

	return got
}
