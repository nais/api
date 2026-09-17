package postgres

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/slug"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewPostgresAccessResource(t *testing.T) {
	expiresAt := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	resource := newPostgresAccessResource(CreatePostgresAccessInput{
		PostgresInstance:         "orders",
		TeamSlug:                 slug.Slug("team-a"),
		EnvironmentName:          "dev",
		AccessLevel:              PostgresAccessLevelReadWrite,
		ClientWireGuardPublicKey: "client-public-key",
	}, "user@example.com", "postgres-access-12345678", expiresAt)

	if got, want := resource.GetAPIVersion(), "nais.io/v1"; got != want {
		t.Errorf("apiVersion = %q, want %q", got, want)
	}
	if got, want := resource.GetKind(), "PostgresAccess"; got != want {
		t.Errorf("kind = %q, want %q", got, want)
	}
	if got, want := resource.GetName(), "postgres-access-12345678"; got != want {
		t.Errorf("name = %q, want %q", got, want)
	}
	if got, want := resource.GetNamespace(), "team-a"; got != want {
		t.Errorf("namespace = %q, want %q", got, want)
	}

	spec, found, err := unstructured.NestedMap(resource.Object, "spec")
	if err != nil || !found {
		t.Fatalf("spec = (%v, %t, %v), want a spec", spec, found, err)
	}
	wantSpec := map[string]any{
		"postgresInstance":         "orders",
		"username":                 "user@example.com",
		"accessLevel":              "readwrite",
		"expiresAt":                "2026-09-17T12:00:00Z",
		"clientWireGuardPublicKey": "client-public-key",
	}
	if !reflect.DeepEqual(wantSpec, spec) {
		t.Errorf("spec = %#v, want %#v", spec, wantSpec)
	}
}

func TestPostgresAccessState(t *testing.T) {
	future := time.Now().Add(time.Hour)
	past := time.Now().Add(-time.Hour)

	tests := []struct {
		name      string
		expiresAt time.Time
		status    map[string]any
		wantState PostgresAccessState
		wantMsg   string
	}{
		{
			name:      "expired",
			expiresAt: past,
			wantState: PostgresAccessStateExpired,
			wantMsg:   "access has expired",
		},
		{
			name:      "pending without status",
			expiresAt: future,
			wantState: PostgresAccessStatePending,
			wantMsg:   "waiting for controller",
		},
		{
			name:      "ready",
			expiresAt: future,
			status: map[string]any{
				"conditions": []any{
					map[string]any{
						"type":    "Ready",
						"status":  "True",
						"message": "Database role and tunnel are ready",
					},
				},
			},
			wantState: PostgresAccessStateReady,
			wantMsg:   "Database role and tunnel are ready",
		},
		{
			name:      "failed unsupported access level",
			expiresAt: future,
			status: map[string]any{
				"conditions": []any{
					map[string]any{
						"type":    "Ready",
						"status":  "False",
						"reason":  "UnsupportedAccessLevel",
						"message": "readwritecreate requires an instance initialized with the app_readwritecreate group role",
					},
				},
			},
			wantState: PostgresAccessStateFailed,
			wantMsg:   "readwritecreate requires an instance initialized with the app_readwritecreate group role",
		},
		{
			name:      "pending waiting on tunnel",
			expiresAt: future,
			status: map[string]any{
				"conditions": []any{
					map[string]any{
						"type":    "Ready",
						"status":  "False",
						"message": "waiting for tunnel",
					},
				},
			},
			wantState: PostgresAccessStatePending,
			wantMsg:   "waiting for tunnel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := map[string]any{}
			if tt.status != nil {
				obj["status"] = tt.status
			}
			gotState, gotMsg := postgresAccessState(obj, tt.expiresAt)
			if gotState != tt.wantState {
				t.Errorf("state = %q, want %q", gotState, tt.wantState)
			}
			if gotMsg != tt.wantMsg {
				t.Errorf("message = %q, want %q", gotMsg, tt.wantMsg)
			}
		})
	}
}

func TestPostgresAccessConnectionDetails(t *testing.T) {
	now := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	ready := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"metadata": map[string]any{"name": "access"},
			"spec":     map[string]any{"expiresAt": "2026-09-17T13:00:00Z"},
			"status": map[string]any{
				"credentialSecretName": "access-credentials",
				"serverName":           "postgres.example",
				"conditions":           []any{map[string]any{"type": "Ready", "status": "True"}},
				"tunnel":               map[string]any{"endpoint": "endpoint:1234", "gatewayPublicKey": "gateway-key"},
			},
		}}
	}

	tests := []struct {
		name string
		edit func(*unstructured.Unstructured)
		want string
	}{
		{name: "ready"},
		{name: "expired", edit: func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, "2026-09-17T12:00:00Z", "spec", "expiresAt")
		}, want: "expired"},
		{name: "not ready", edit: func(u *unstructured.Unstructured) {
			_ = unstructured.SetNestedField(u.Object, []any{map[string]any{"type": "Ready", "status": "False"}}, "status", "conditions")
		}, want: "not ready"},
		{name: "missing secret name", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "credentialSecretName")
		}, want: "not ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := ready()
			if tt.edit != nil {
				tt.edit(u)
			}
			got, secretName, err := postgresAccessConnectionDetails(u, now)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("error = %v, want %q", err, tt.want)
				}
				return
			}
			if err != nil {
				t.Fatalf("postgresAccessConnectionDetails: %v", err)
			}
			if secretName != "access-credentials" {
				t.Errorf("secret name = %q", secretName)
			}
			if got.ServerName != "postgres.example" || got.Tunnel.Endpoint != "endpoint:1234" || got.Tunnel.GatewayPublicKey != "gateway-key" {
				t.Errorf("connection = %#v", got)
			}
		})
	}
}

func TestPostgresAccessConnectionSecret(t *testing.T) {
	secret := &corev1.Secret{Data: map[string][]byte{
		corev1.BasicAuthPasswordKey: []byte("supersecret"),
		"ca.crt":                    []byte("test-ca-certificate"),
	}}
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}
	password, ca, err := postgresAccessConnectionSecret(&unstructured.Unstructured{Object: u})
	if err != nil {
		t.Fatalf("postgresAccessConnectionSecret: %v", err)
	}
	if password != "supersecret" || ca != "test-ca-certificate" {
		t.Errorf("got password=%q ca=%q", password, ca)
	}

	delete(secret.Data, "ca.crt")
	u, err = runtime.DefaultUnstructuredConverter.ToUnstructured(secret)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}
	if _, _, err := postgresAccessConnectionSecret(&unstructured.Unstructured{Object: u}); err == nil {
		t.Fatal("missing ca.crt did not fail")
	}
}
