package postgres

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewPostgresAccessResource(t *testing.T) {
	expiresAt := time.Date(2026, time.September, 17, 12, 0, 0, 0, time.UTC)
	resource := newPostgresAccessResource(CreatePostgresAccessInput{
		PostgresBranch:  "orders",
		TeamSlug:        slug.Slug("team-a"),
		EnvironmentName: "dev",
		AccessLevel:     PostgresAccessLevelReadWrite,
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
		"postgresBranch": "orders",
		"username":       "user@example.com",
		"accessLevel":    "readwrite",
		"expiresAt":      "2026-09-17T12:00:00Z",
	}
	if !reflect.DeepEqual(wantSpec, spec) {
		t.Errorf("spec = %#v, want %#v", spec, wantSpec)
	}
}

func TestCreatePostgresAccessTTL(t *testing.T) {
	tests := []struct {
		name    string
		ttl     string
		want    time.Duration
		wantErr string
	}{
		{name: "default", want: time.Hour},
		{name: "requested", ttl: "30m", want: 30 * time.Minute},
		{name: "maximum", ttl: "1h", want: time.Hour},
		{name: "invalid", ttl: "tomorrow", wantErr: "TTL must be a Go duration"},
		{name: "zero", ttl: "0s", wantErr: "TTL must be positive"},
		{name: "too long", ttl: "1h1m", wantErr: "TTL cannot exceed 1h0m0s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := (CreatePostgresAccessInput{TTL: tt.ttl}).accessTTL()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("accessTTL() error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("accessTTL() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("accessTTL() = %s, want %s", got, tt.want)
			}
		})
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
						"message": "database role and relay mapping are ready",
					},
				},
			},
			wantState: PostgresAccessStateReady,
			wantMsg:   "database role and relay mapping are ready",
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
			name:      "pending waiting on relay",
			expiresAt: future,
			status: map[string]any{
				"conditions": []any{
					map[string]any{
						"type":    "Ready",
						"status":  "False",
						"message": "waiting for relay",
					},
				},
			},
			wantState: PostgresAccessStatePending,
			wantMsg:   "waiting for relay",
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
				"databaseRole":   "personal-role",
				"relayAccess":    "access",
				"tokenSecret":    "access-relay-token",
				"serverName":     "pg-orders-rw.team.svc.cluster.local",
				"serverCASecret": "pg-orders-ca",
				"conditions":     []any{map[string]any{"type": "Ready", "status": "True"}},
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
		{name: "missing token secret name", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "tokenSecret")
		}, want: "not ready"},
		{name: "missing relay mapping", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "relayAccess")
		}, want: "not ready"},
		{name: "missing server name", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "serverName")
		}, want: "not ready"},
		{name: "missing server CA reference", edit: func(u *unstructured.Unstructured) {
			unstructured.RemoveNestedField(u.Object, "status", "serverCASecret")
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
			if secretName != "access-relay-token" {
				t.Errorf("secret name = %q", secretName)
			}
			if got == nil {
				t.Fatal("connection is nil")
			}
		})
	}
}
