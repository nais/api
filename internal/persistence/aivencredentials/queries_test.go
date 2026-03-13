package aivencredentials

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/sirupsen/logrus/hooks/test"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
)

func TestParseTTL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{name: "1 day", input: "1d", want: 24 * time.Hour},
		{name: "7 days", input: "7d", want: 7 * 24 * time.Hour},
		{name: "30 days", input: "30d", want: 30 * 24 * time.Hour},
		{name: "24 hours", input: "24h", want: 24 * time.Hour},
		{name: "168 hours", input: "168h", want: 168 * time.Hour},
		{name: "with whitespace", input: "  3d  ", want: 3 * 24 * time.Hour},
		{name: "exceeds max", input: "31d", wantErr: true},
		{name: "zero days", input: "0d", wantErr: true},
		{name: "negative days", input: "-1d", wantErr: true},
		{name: "zero hours", input: "0h", wantErr: true},
		{name: "negative hours", input: "-1h", wantErr: true},
		{name: "invalid format", input: "abc", wantErr: true},
		{name: "empty string", input: "", wantErr: true},
		{name: "exceeds max hours", input: "721h", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTTL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("parseTTL(%q) = %v, want error", tt.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("parseTTL(%q) error = %v", tt.input, err)
				return
			}
			if got != tt.want {
				t.Errorf("parseTTL(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateSecretName(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		namespace string
		service   string
	}{
		{name: "opensearch", username: "user@example.com", namespace: "my-team", service: "opensearch"},
		{name: "valkey", username: "user@example.com", namespace: "my-team", service: "valkey"},
		{name: "kafka", username: "user@example.com", namespace: "my-team", service: "kafka"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateSecretName(tt.username, tt.namespace, tt.service)

			// Must start with "aiven-<service>-"
			prefix := "aiven-" + tt.service + "-"
			if len(got) < len(prefix) || got[:len(prefix)] != prefix {
				t.Errorf("generateSecretName() = %q, want prefix %q", got, prefix)
			}

			// Must be deterministic
			got2 := generateSecretName(tt.username, tt.namespace, tt.service)
			if got != got2 {
				t.Errorf("generateSecretName() not deterministic: %q != %q", got, got2)
			}
		})
	}

	// Different inputs must produce different names
	a := generateSecretName("user1@example.com", "team-a", "opensearch")
	b := generateSecretName("user2@example.com", "team-a", "opensearch")
	if a == b {
		t.Errorf("different users produced same secret name: %q", a)
	}
}

func TestGenerateAppName(t *testing.T) {
	tests := []struct {
		name     string
		username string
		service  string
	}{
		{name: "opensearch", username: "user@example.com", service: "opensearch"},
		{name: "valkey", username: "user@example.com", service: "valkey"},
		{name: "kafka", username: "user@example.com", service: "kafka"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateAppName(tt.username, tt.service)

			// Must start with "console-<service>-"
			prefix := "console-" + tt.service + "-"
			if len(got) < len(prefix) || got[:len(prefix)] != prefix {
				t.Errorf("generateAppName() = %q, want prefix %q", got, prefix)
			}

			// Must be deterministic
			got2 := generateAppName(tt.username, tt.service)
			if got != got2 {
				t.Errorf("generateAppName() not deterministic: %q != %q", got, got2)
			}
		})
	}

	// Different inputs must produce different names
	a := generateAppName("user1@example.com", "opensearch")
	b := generateAppName("user2@example.com", "opensearch")
	if a == b {
		t.Errorf("different users produced same app name: %q", a)
	}

	// Verify dead code was removed: generateAppName should NOT contain username
	// (old bug had ReplaceAll on name but then didn't use it)
	got := generateAppName("user.name@example.com", "opensearch")
	prefix := "console-opensearch-"
	if len(got) < len(prefix) || got[:len(prefix)] != prefix {
		t.Errorf("generateAppName() = %q, want prefix %q", got, prefix)
	}
}

func TestSecretData(t *testing.T) {
	logger, hook := test.NewNullLogger()

	tests := []struct {
		name     string
		object   map[string]any
		want     map[string]string
		wantLogs int
	}{
		{
			name: "extracts string values",
			object: map[string]any{
				"data": map[string]any{
					"KEY1": "value1",
					"KEY2": "value2",
				},
			},
			want:     map[string]string{"KEY1": "value1", "KEY2": "value2"},
			wantLogs: 0,
		},
		{
			name:     "handles missing data field",
			object:   map[string]any{},
			want:     map[string]string{},
			wantLogs: 0,
		},
		{
			name: "logs non-string values",
			object: map[string]any{
				"data": map[string]any{
					"KEY1": "value1",
					"KEY2": 42,
				},
			},
			want:     map[string]string{"KEY1": "value1"},
			wantLogs: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook.Reset()
			secret := &unstructured.Unstructured{Object: tt.object}
			got := secretData(secret, logger)

			if len(got) != len(tt.want) {
				t.Errorf("secretData() returned %d entries, want %d", len(got), len(tt.want))
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("secretData()[%q] = %q, want %q", k, got[k], v)
				}
			}
			if len(hook.Entries) != tt.wantLogs {
				t.Errorf("secretData() logged %d warnings, want %d", len(hook.Entries), tt.wantLogs)
			}
		})
	}
}

func newFakeDynamicClient(objects ...runtime.Object) *dynfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	// Register the GVKs we need
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "aiven.nais.io", Version: "v1", Kind: "AivenApplication"},
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "aiven.nais.io", Version: "v1", Kind: "AivenApplicationList"},
		&unstructured.UnstructuredList{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"},
		&unstructured.Unstructured{},
	)
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "", Version: "v1", Kind: "SecretList"},
		&unstructured.UnstructuredList{},
	)
	return dynfake.NewSimpleDynamicClient(scheme, objects...)
}

func TestWaitForSecret(t *testing.T) {
	t.Run("returns secret when it exists", func(t *testing.T) {
		secret := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata": map[string]any{
					"name":      "test-secret",
					"namespace": "my-team",
				},
				"data": map[string]any{
					"KEY": "value",
				},
			},
		}
		client := newFakeDynamicClient(secret)
		ctx := context.Background()

		got, err := waitForSecret(ctx, client, "my-team", "test-secret")
		if err != nil {
			t.Fatalf("waitForSecret() error = %v", err)
		}
		if got.GetName() != "test-secret" {
			t.Errorf("waitForSecret() name = %q, want %q", got.GetName(), "test-secret")
		}
	})

	t.Run("returns error on context cancellation", func(t *testing.T) {
		client := newFakeDynamicClient()
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		_, err := waitForSecret(ctx, client, "my-team", "nonexistent")
		if err == nil {
			t.Fatal("waitForSecret() expected error for cancelled context")
		}
	})
}

func TestCreateOrUpdateAivenApplication(t *testing.T) {
	t.Run("creates new AivenApplication", func(t *testing.T) {
		client := newFakeDynamicClient()
		ctx := context.Background()

		spec := map[string]any{
			"protected": true,
			"expiresAt": "2025-01-01T00:00:00Z",
		}

		actor := &fakeUser{identity: "test@example.com"}
		err := createOrUpdateAivenApplication(ctx, client, "test-app", "my-team", spec, actor)
		if err != nil {
			t.Fatalf("createOrUpdateAivenApplication() error = %v", err)
		}

		// Verify it was created
		got, err := client.Resource(aivenApplicationGVR).Namespace("my-team").Get(ctx, "test-app", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if got.GetName() != "test-app" {
			t.Errorf("name = %q, want %q", got.GetName(), "test-app")
		}
		if got.GetNamespace() != "my-team" {
			t.Errorf("namespace = %q, want %q", got.GetNamespace(), "my-team")
		}
	})

	t.Run("updates existing AivenApplication without owner references", func(t *testing.T) {
		existing := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "aiven.nais.io/v1",
				"kind":       "AivenApplication",
				"metadata": map[string]any{
					"name":      "test-app",
					"namespace": "my-team",
				},
				"spec": map[string]any{
					"protected": true,
					"expiresAt": "2025-01-01T00:00:00Z",
				},
			},
		}
		client := newFakeDynamicClient(existing)
		ctx := context.Background()

		newSpec := map[string]any{
			"protected": true,
			"expiresAt": "2026-01-01T00:00:00Z",
		}

		actor := &fakeUser{identity: "test@example.com"}
		err := createOrUpdateAivenApplication(ctx, client, "test-app", "my-team", newSpec, actor)
		if err != nil {
			t.Fatalf("createOrUpdateAivenApplication() error = %v", err)
		}

		// Verify it was updated
		got, err := client.Resource(aivenApplicationGVR).Namespace("my-team").Get(ctx, "test-app", metav1.GetOptions{})
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		gotSpec, _, _ := unstructured.NestedMap(got.Object, "spec")
		if gotSpec["expiresAt"] != "2026-01-01T00:00:00Z" {
			t.Errorf("spec.expiresAt = %v, want %q", gotSpec["expiresAt"], "2026-01-01T00:00:00Z")
		}
	})

	t.Run("refuses to overwrite owned AivenApplication", func(t *testing.T) {
		existing := &unstructured.Unstructured{
			Object: map[string]any{
				"apiVersion": "aiven.nais.io/v1",
				"kind":       "AivenApplication",
				"metadata": map[string]any{
					"name":      "test-app",
					"namespace": "my-team",
					"ownerReferences": []any{
						map[string]any{
							"apiVersion": "nais.io/v1alpha1",
							"kind":       "Application",
							"name":       "my-app",
							"uid":        "abc-123",
						},
					},
				},
				"spec": map[string]any{},
			},
		}
		client := newFakeDynamicClient(existing)
		ctx := context.Background()

		actor := &fakeUser{identity: "test@example.com"}
		err := createOrUpdateAivenApplication(ctx, client, "test-app", "my-team", map[string]any{}, actor)
		if err == nil {
			t.Fatal("createOrUpdateAivenApplication() expected error for owned resource")
		}
	})
}

// fakeUser implements authz.AuthenticatedUser for tests.
type fakeUser struct {
	identity string
}

var _ authz.AuthenticatedUser = &fakeUser{}

func (f *fakeUser) Identity() string                                  { return f.identity }
func (f *fakeUser) GetID() uuid.UUID                                  { return uuid.Nil }
func (f *fakeUser) IsServiceAccount() bool                            { return false }
func (f *fakeUser) IsAdmin() bool                                     { return false }
func (f *fakeUser) GCPTeamGroups(_ context.Context) ([]string, error) { return nil, nil }
