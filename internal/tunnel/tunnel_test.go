package tunnel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/slug"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPhaseConstantsAreDefined(t *testing.T) {
	phases := []Phase{
		PhasePending,
		PhaseProvisioning,
		PhaseReady,
		PhaseConnected,
		PhaseFailed,
		PhaseTerminated,
	}
	for _, p := range phases {
		if p == "" {
			t.Errorf("phase constant is empty string")
		}
	}
}

func TestPhaseConstantValues(t *testing.T) {
	cases := []struct {
		phase    Phase
		expected string
	}{
		{PhasePending, "Pending"},
		{PhaseProvisioning, "Provisioning"},
		{PhaseReady, "Ready"},
		{PhaseConnected, "Connected"},
		{PhaseFailed, "Failed"},
		{PhaseTerminated, "Terminated"},
	}
	for _, c := range cases {
		if string(c.phase) != c.expected {
			t.Errorf("Phase %q: expected %q", c.phase, c.expected)
		}
	}
}

func TestErrorsAreNonNil(t *testing.T) {
	errs := []error{
		ErrTunnelNotFound,
		ErrTunnelNotReady,
		ErrNotImplemented,
	}
	for _, err := range errs {
		if err == nil {
			t.Errorf("expected non-nil error, got nil")
		}
		// errors.Is with itself must return true
		if !errors.Is(err, err) {
			t.Errorf("error %v does not satisfy errors.Is with itself", err)
		}
	}
}

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

func TestTunnelStructFields(t *testing.T) {
	now := time.Now()
	tun := Tunnel{
		TunnelID:            "uid-123",
		Name:                "tunnel-abc",
		TeamSlug:            "my-team",
		Environment:         "dev",
		Target:              Target{Host: "db.internal", Port: 5432},
		ClientPublicKey:     "client-pub-key",
		ClientSTUNEndpoint:  "1.2.3.4:12345",
		GatewayPublicKey:    "gw-pub-key",
		GatewaySTUNEndpoint: "5.6.7.8:54321",
		GatewayPodName:      "gateway-pod-0",
		Phase:               PhaseReady,
		Message:             "all good",
		CreatedAt:           now,
	}

	if tun.TunnelID != "uid-123" {
		t.Errorf("TunnelID: got %q, want %q", tun.TunnelID, "uid-123")
	}
	if tun.Target.Host != "db.internal" {
		t.Errorf("Target.Host: got %q, want %q", tun.Target.Host, "db.internal")
	}
	if tun.Target.Port != 5432 {
		t.Errorf("Target.Port: got %d, want %d", tun.Target.Port, 5432)
	}
	if tun.Phase != PhaseReady {
		t.Errorf("Phase: got %q, want %q", tun.Phase, PhaseReady)
	}
	if !tun.CreatedAt.Equal(now) {
		t.Errorf("CreatedAt mismatch")
	}
}

func TestTunnelIDMethod(t *testing.T) {
	tun := Tunnel{TunnelID: "my-uid"}
	id := tun.ID()
	if id.ID != "my-uid" {
		t.Errorf("ID.ID: got %q, want %q", id.ID, "my-uid")
	}
	if id.Type != "Tunnel" {
		t.Errorf("ID.Type: got %q, want %q", id.Type, "Tunnel")
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

func TestActivityLogEntryTypes(t *testing.T) {
	created := TunnelCreatedActivityLogEntry{
		GenericActivityLogEntry: activitylog.GenericActivityLogEntry{
			Actor:        "user@example.com",
			Message:      "Created Tunnel",
			ResourceType: ActivityLogEntryResourceTypeTunnel,
			ResourceName: "tunnel-abc",
		},
		TunnelID:          "uid-123",
		TeamSlugForTunnel: slug.Slug("my-team"),
		TargetHost:        "db.internal",
	}
	if created.TunnelID != "uid-123" {
		t.Errorf("TunnelCreatedActivityLogEntry.TunnelID: got %q, want %q", created.TunnelID, "uid-123")
	}
	if string(created.TeamSlugForTunnel) != "my-team" {
		t.Errorf("TunnelCreatedActivityLogEntry.TeamSlugForTunnel: got %q, want %q", created.TeamSlugForTunnel, "my-team")
	}

	deleted := TunnelDeletedActivityLogEntry{
		GenericActivityLogEntry: activitylog.GenericActivityLogEntry{
			Actor:        "user@example.com",
			Message:      "Deleted Tunnel",
			ResourceType: ActivityLogEntryResourceTypeTunnel,
			ResourceName: "tunnel-abc",
		},
		TunnelID:          "uid-456",
		TeamSlugForTunnel: slug.Slug("another-team"),
	}
	if deleted.TunnelID != "uid-456" {
		t.Errorf("TunnelDeletedActivityLogEntry.TunnelID: got %q, want %q", deleted.TunnelID, "uid-456")
	}
}

func TestActivityLogResourceType(t *testing.T) {
	if ActivityLogEntryResourceTypeTunnel != "TUNNEL" {
		t.Errorf("ActivityLogEntryResourceTypeTunnel: got %q, want %q", ActivityLogEntryResourceTypeTunnel, "TUNNEL")
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
				"phase":               "Ready",
				"gatewayPublicKey":    "gw-key",
				"gatewaySTUNEndpoint": "10.0.0.1:9000",
				"gatewayPodName":      "gateway-0",
				"message":             "connected",
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
	// Fields not in spec/status should be zero values
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

func TestCreateTunnelInputFields(t *testing.T) {
	input := CreateTunnelInput{
		TeamSlug:        "team-c",
		EnvironmentName: "staging",
		InstanceName:    "my-instance",
		TargetHost:      "pg.internal",
		TargetPort:      5432,
		ClientPublicKey: "pub-key",
	}
	if input.TeamSlug != "team-c" {
		t.Errorf("TeamSlug: got %q, want %q", input.TeamSlug, "team-c")
	}
	if input.TargetPort != 5432 {
		t.Errorf("TargetPort: got %d, want %d", input.TargetPort, 5432)
	}
}

func TestDeleteTunnelPayload(t *testing.T) {
	payload := DeleteTunnelPayload{Success: true}
	if !payload.Success {
		t.Errorf("DeleteTunnelPayload.Success: expected true")
	}
}
