package instancegroup

import (
	"context"
	"testing"

	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// makeIG creates a minimal InstanceGroup for testing.
func makeIG(appName string, envVars []corev1.EnvVar) *InstanceGroup {
	return &InstanceGroup{
		ApplicationName: appName,
		TeamSlug:        "my-team",
		EnvironmentName: "dev",
		PodTemplateSpec: corev1.PodTemplateSpec{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name: appName,
						Env:  envVars,
					},
				},
			},
		},
	}
}

// TestSpecOrNais verifies the specOrNais helper function.
func TestSpecOrNais(t *testing.T) {
	tests := []struct {
		name        string
		envName     string
		userDefined map[string]struct{}
		want        InstanceGroupValueSourceKind
	}{
		{
			name:        "name in user-defined set returns SPEC",
			envName:     "MY_VAR",
			userDefined: map[string]struct{}{"MY_VAR": {}},
			want:        InstanceGroupValueSourceKindSpec,
		},
		{
			name:        "name not in user-defined set returns NAIS",
			envName:     "INJECTED_VAR",
			userDefined: map[string]struct{}{"MY_VAR": {}},
			want:        InstanceGroupValueSourceKindNais,
		},
		{
			name:        "nil user-defined set (Application CRD not found) returns NAIS",
			envName:     "ANY_VAR",
			userDefined: nil,
			want:        InstanceGroupValueSourceKindNais,
		},
		{
			name:        "empty user-defined set returns NAIS",
			envName:     "ANY_VAR",
			userDefined: map[string]struct{}{},
			want:        InstanceGroupValueSourceKindNais,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := specOrNais(tt.envName, tt.userDefined)
			if got != tt.want {
				t.Errorf("specOrNais(%q, ...) = %v, want %v", tt.envName, got, tt.want)
			}
		})
	}
}

// TestUserDefinedEnvNames verifies extraction of env var names from Application CRD.
func TestUserDefinedEnvNames(t *testing.T) {
	app := &nais_io_v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{Name: "my-app", Namespace: "my-team"},
		Spec: nais_io_v1alpha1.ApplicationSpec{
			Env: nais_io_v1.EnvVars{
				{Name: "MY_VAR", Value: "hello"},
				{Name: "OTHER_VAR", Value: "world"},
			},
		},
	}

	names := userDefinedEnvNames(app)

	if _, ok := names["MY_VAR"]; !ok {
		t.Error("expected MY_VAR in user-defined env names")
	}
	if _, ok := names["OTHER_VAR"]; !ok {
		t.Error("expected OTHER_VAR in user-defined env names")
	}
	if _, ok := names["INJECTED"]; ok {
		t.Error("did not expect INJECTED in user-defined env names")
	}
}

// TestUserDefinedEnvNamesNilApp verifies nil Application returns nil map.
func TestUserDefinedEnvNamesNilApp(t *testing.T) {
	names := userDefinedEnvNames(nil)
	if names != nil {
		t.Errorf("expected nil for nil app, got %v", names)
	}
}

// TestFieldRefAndResourceFieldRefAlwaysNais verifies that fieldRef and resourceFieldRef
// env vars are always classified as NAIS regardless of Application CRD.
func TestFieldRefAndResourceFieldRefAlwaysNais(t *testing.T) {
	fieldPath := "metadata.name"
	resource := "limits.cpu"

	envVars := []corev1.EnvVar{
		{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{FieldPath: fieldPath},
			},
		},
		{
			Name: "CPU_LIMIT",
			ValueFrom: &corev1.EnvVarSource{
				ResourceFieldRef: &corev1.ResourceFieldSelector{Resource: resource},
			},
		},
	}

	ig := makeIG("my-app", envVars)

	// Use a loaders with a nil appWatcher - fieldRef/resourceFieldRef should always be NAIS.
	l := &loaders{
		log: logrus.New(),
	}
	ctx := context.WithValue(context.Background(), loadersKey, l)

	result, err := ListEnvironmentVariables(ctx, ig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 env vars, got %d", len(result))
	}

	for _, ev := range result {
		if ev.Source.Kind != InstanceGroupValueSourceKindNais {
			t.Errorf("env var %q: expected NAIS source kind, got %v", ev.Name, ev.Source.Kind)
		}
	}

	if result[0].Name != "POD_NAME" {
		t.Errorf("expected POD_NAME, got %s", result[0].Name)
	}
	if result[1].Name != "CPU_LIMIT" {
		t.Errorf("expected CPU_LIMIT, got %s", result[1].Name)
	}
}

// TestProjectedVolumeSubPathFallbackIsNais verifies that projected volumes with subPath
// that have no secret/configmap source are classified as NAIS (not SPEC).
func TestProjectedVolumeSubPathFallbackIsNais(t *testing.T) {
	projected := &corev1.ProjectedVolumeSource{
		Sources: []corev1.VolumeProjection{
			// No secret or configmap - only service account token
			{
				ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
					Path: "token",
				},
			},
		},
	}

	mount := corev1.VolumeMount{
		MountPath: "/var/run/secrets/token",
		SubPath:   "token",
	}

	l := &loaders{log: logrus.New()}
	ig := &InstanceGroup{ApplicationName: "my-app", TeamSlug: "my-team", EnvironmentName: "dev"}

	files := expandProjectedVolume(context.Background(), l, ig, mount, projected)

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if files[0].Source.Kind != InstanceGroupValueSourceKindNais {
		t.Errorf("expected NAIS source kind for projected volume fallback, got %v", files[0].Source.Kind)
	}
	if files[0].Source.Name != "projected" {
		t.Errorf("expected source name 'projected', got %q", files[0].Source.Name)
	}
}

// TestInlineEnvVarClassificationWithNilAppWatcher verifies that when the Application CRD
// cannot be found (nil appWatcher), inline env vars are classified as NAIS — we cannot
// confirm the user defined them, so we default to the platform-injected classification.
func TestInlineEnvVarClassificationWithNilAppWatcher(t *testing.T) {
	envVars := []corev1.EnvVar{
		{Name: "SOME_VAR", Value: "value"},
	}

	ig := makeIG("my-app", envVars)

	// nil appWatcher means getApplicationSpec returns nil, which means specOrNais returns NAIS
	l := &loaders{
		log: logrus.New(),
	}
	ctx := context.WithValue(context.Background(), loadersKey, l)

	result, err := ListEnvironmentVariables(ctx, ig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 env var, got %d", len(result))
	}

	if result[0].Source.Kind != InstanceGroupValueSourceKindNais {
		t.Errorf("expected NAIS when Application CRD not found, got %v", result[0].Source.Kind)
	}
}
