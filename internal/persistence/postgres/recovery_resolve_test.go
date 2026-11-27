package postgres

import (
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/kubernetes"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

var postgresTestGVR = schema.GroupVersionResource{Group: "nais.io", Version: "v1", Resource: "postgreses"}

func recoverySource(t *testing.T, mutate func(*nais_io_v1.Postgres)) *unstructured.Unstructured {
	t.Helper()
	source := &nais_io_v1.Postgres{
		TypeMeta:   metav1.TypeMeta{Kind: "Postgres", APIVersion: "nais.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: "source", Namespace: "team", UID: "source-uid"},
		Spec: nais_io_v1.PostgresSpec{
			MajorVersion:     "18",
			Extensions:       []nais_io_v1.PostgresExtension{{Name: "postgis"}},
			Resources:        nais_io_v1.PostgresResources{Cpu: resource.MustParse("500m"), Memory: resource.MustParse("2Gi"), DiskSize: resource.MustParse("20Gi")},
			HighAvailability: true,
		},
		Status: &nais_io_v1.PostgresStatus{ClusterName: "pg-source"},
	}
	if mutate != nil {
		mutate(source)
	}
	object, err := kubernetes.ToUnstructured(source)
	if err != nil {
		t.Fatal(err)
	}
	return object
}

func TestResolveRecoveryRules(t *testing.T) {
	ptr := func(s string) *string { return &s }
	extensions := []string{"postgis"}
	for _, tt := range []struct {
		name      string
		mutate    func(*nais_io_v1.Postgres)
		input     func(*CreatePostgresInput)
		wantError string
	}{
		{name: "source not found", input: func(i *CreatePostgresInput) { i.Recovery.SourcePostgres = "missing" }, wantError: "not found"},
		{name: "source deleting", mutate: func(p *nais_io_v1.Postgres) { now := metav1.Now(); p.DeletionTimestamp = &now }, wantError: "deleting"},
		{name: "source without cluster", mutate: func(p *nais_io_v1.Postgres) { p.Status = nil }, wantError: "no recoverable cluster"},
		{name: "major version mismatch", input: func(i *CreatePostgresInput) { i.MajorVersion = "17" }, wantError: "major version"},
		{name: "source extension omitted", input: func(i *CreatePostgresInput) { i.Extensions = []string{"pg_trgm"} }, wantError: "source extension"},
		{name: "disk smaller than source", input: func(i *CreatePostgresInput) { i.DiskSize = ptr("10Gi") }, wantError: "smaller than the source"},
		{name: "explicit equal disk accepted until archive lookup", input: func(i *CreatePostgresInput) { i.DiskSize = ptr("20Gi"); i.Extensions = extensions }, wantError: "-"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			client := fake.NewSimpleDynamicClient(scheme, recoverySource(t, tt.mutate)).Resource(postgresTestGVR).Namespace("team")
			input := CreatePostgresInput{Name: "restored", EnvironmentName: "dev", MajorVersion: "18", Recovery: &PostgresRecoveryInput{SourcePostgres: "source", TargetTime: time.Now().Add(-time.Hour).Truncate(time.Second)}}
			if tt.input != nil {
				tt.input(&input)
			}
			target := &nais_io_v1.Postgres{Spec: nais_io_v1.PostgresSpec{MajorVersion: input.MajorVersion, Extensions: postgresExtensions(input.Extensions)}}
			err := func() (err error) {
				defer func() {
					// Rules after this point need the cluster-scoped watcher from context.
					if recover() != nil {
						err = nil
					}
				}()
				_, err = resolveRecovery(t.Context(), client, input, target)
				return err
			}()
			if tt.wantError == "-" {
				if err != nil && !strings.Contains(err.Error(), "context") {
					t.Fatalf("unexpected early rejection: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
		})
	}
}

func TestRecoveryInheritanceAndExplicitOverrides(t *testing.T) {
	source, err := kubernetes.ToConcrete[nais_io_v1.Postgres](recoverySource(t, nil))
	if err != nil {
		t.Fatal(err)
	}
	// resolveRecovery seeds the source configuration; createPostgres then
	// applies only the quantities the user supplied.
	target := &nais_io_v1.Postgres{}
	target.Spec.Resources = *source.Spec.Resources.DeepCopy()
	cpu, memory, disk := "2", "8Gi", "40Gi"
	if err := applyResources(&target.Spec.Resources, &cpu, &memory, &disk); err != nil {
		t.Fatal(err)
	}
	for field, want := range map[string]string{"cpu": "2", "memory": "8Gi", "disk": "40Gi"} {
		got := map[string]resource.Quantity{"cpu": target.Spec.Resources.Cpu, "memory": target.Spec.Resources.Memory, "disk": target.Spec.Resources.DiskSize}[field]
		if got.Cmp(resource.MustParse(want)) != 0 {
			t.Errorf("%s = %s, want explicit %s", field, got.String(), want)
		}
	}
	target.Spec.Resources = *source.Spec.Resources.DeepCopy()
	if err := applyResources(&target.Spec.Resources, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if target.Spec.Resources.DiskSize.Cmp(resource.MustParse("20Gi")) != 0 || target.Spec.Resources.Cpu.Cmp(resource.MustParse("500m")) != 0 {
		t.Errorf("omitted values must inherit the source: %+v", target.Spec.Resources)
	}
}
