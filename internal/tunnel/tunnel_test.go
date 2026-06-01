package tunnel

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestErrorMessages(t *testing.T) {
	cases := []struct {
		err     error
		message string
	}{
		{ErrTunnelNotFound, "tunnel not found"},
		{ErrTunnelNotReady, "tunnel not ready"},
		{ErrNotImplemented, "not implemented"},
	}
	for _, c := range cases {
		if c.err.Error() != c.message {
			t.Errorf("error message: got %q, want %q", c.err.Error(), c.message)
		}
	}
}

func TestTunnelIDMethod(t *testing.T) {
	tun := Tunnel{TeamSlug: "my-team", Environment: "dev", Name: "my-tunnel"}
	id := tun.ID()
	if id.Type != "TU" {
		t.Errorf("ID.Type: got %q, want %q", id.Type, "TU")
	}
	if id.ID == "" {
		t.Errorf("ID.ID: expected non-empty ident string")
	}
}

func TestTunnelGetters(t *testing.T) {
	tun := &Tunnel{Name: "my-tunnel", TeamSlug: "team-a"}
	if tun.GetName() != "my-tunnel" {
		t.Errorf("GetName: got %q, want %q", tun.GetName(), "my-tunnel")
	}
	if tun.GetNamespace() != "team-a" {
		t.Errorf("GetNamespace: got %q, want %q", tun.GetNamespace(), "team-a")
	}
	if tun.GetLabels() != nil {
		t.Errorf("GetLabels: expected nil")
	}
	if tun.DeepCopyObject() != tun {
		t.Errorf("DeepCopyObject: expected same pointer")
	}
}

func TestConverterBasic(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "tunnel-xyz",
				"uid":  "some-uid",
			},
			"spec": map[string]any{
				"teamSlug":        "team-b",
				"environment":     "prod",
				"clientPublicKey": "client-key",
				"target": map[string]any{
					"host": "redis.internal",
					"port": float64(6379),
				},
			},
			"status": map[string]any{
				"phase":             "Ready",
				"gatewayPublicKey":  "gw-key",
				"forwarderEndpoint": "10.0.0.1:9000",
				"gatewayPodName":    "gateway-0",
				"message":           "connected",
			},
		},
	}

	tun, err := converter(u)
	if err != nil {
		t.Fatalf("converter returned unexpected error: %v", err)
	}
	if tun.Name != "tunnel-xyz" {
		t.Errorf("Name: got %q, want %q", tun.Name, "tunnel-xyz")
	}
	if tun.TeamSlug != "team-b" {
		t.Errorf("TeamSlug: got %q, want %q", tun.TeamSlug, "team-b")
	}
	if tun.Target.Host != "redis.internal" {
		t.Errorf("Target.Host: got %q, want %q", tun.Target.Host, "redis.internal")
	}
	if tun.Target.Port != 6379 {
		t.Errorf("Target.Port: got %d, want %d", tun.Target.Port, 6379)
	}
	if tun.Phase != PhaseReady {
		t.Errorf("Phase: got %q, want %q", tun.Phase, PhaseReady)
	}
	if tun.GatewayPublicKey != "gw-key" {
		t.Errorf("GatewayPublicKey: got %q, want %q", tun.GatewayPublicKey, "gw-key")
	}
	if tun.ForwarderEndpoint != "10.0.0.1:9000" {
		t.Errorf("ForwarderEndpoint: got %q, want %q", tun.ForwarderEndpoint, "10.0.0.1:9000")
	}
	if tun.GatewayPodName != "gateway-0" {
		t.Errorf("GatewayPodName: got %q, want %q", tun.GatewayPodName, "gateway-0")
	}
	if tun.Message != "connected" {
		t.Errorf("Message: got %q, want %q", tun.Message, "connected")
	}
}

func TestConverterEmptyObject(t *testing.T) {
	u := &unstructured.Unstructured{
		Object: map[string]any{
			"metadata": map[string]any{
				"name": "tunnel-empty",
			},
		},
	}
	tun, err := converter(u)
	if err != nil {
		t.Fatalf("converter returned unexpected error: %v", err)
	}
	if tun.Name != "tunnel-empty" {
		t.Errorf("Name: got %q, want %q", tun.Name, "tunnel-empty")
	}
	if tun.Target.Host != "" {
		t.Errorf("Target.Host: expected empty, got %q", tun.Target.Host)
	}
	if tun.Phase != "" {
		t.Errorf("Phase: expected empty, got %q", tun.Phase)
	}
}

func TestFromContextNoLoaders(t *testing.T) {
	ctx := context.Background()
	loaders := FromContext(ctx)
	if loaders != nil {
		t.Errorf("expected nil loaders from empty context, got %v", loaders)
	}
}

func TestWithLoaders(t *testing.T) {
	loaders := &Loaders{}
	ctx := WithLoaders(context.Background(), loaders)
	got := FromContext(ctx)
	if got != loaders {
		t.Errorf("FromContext: got %v, want %v", got, loaders)
	}
}
