package aiven_test

import (
	"context"
	"testing"

	"github.com/nais/api/internal/thirdparty/aiven"
)

// A fake that reports both versions for every service hides the mistake worth catching: reading a
// Valkey version off an OpenSearch service, or the other way round.
func TestFakeAivenClientServiceGet(t *testing.T) {
	client := aiven.NewFakeAivenClient()

	tests := []struct {
		name        string
		serviceName string
		wantKey     string
		wantAbsent  string
		wantMissing bool
	}{
		{
			name:        "valkey service",
			serviceName: "valkey-myteam-cache",
			wantKey:     "valkey_version", wantAbsent: "opensearch_version",
		},
		{
			name:        "opensearch service",
			serviceName: "opensearch-myteam-logs",
			wantKey:     "opensearch_version", wantAbsent: "valkey_version",
		},
		{
			name:        "service name without a known prefix",
			serviceName: "myteam-cache",
			wantMissing: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := client.ServiceGet(context.Background(), "", tt.serviceName)

			if tt.wantMissing {
				if !aiven.IsNotFound(err) {
					t.Fatalf("err = %v, want not found", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ServiceGet() error = %v", err)
			}
			if v, ok := got.Metadata[tt.wantKey].(string); !ok || v == "" {
				t.Errorf("metadata[%q] = %v, want a version string", tt.wantKey, got.Metadata[tt.wantKey])
			}
			if _, present := got.Metadata[tt.wantAbsent]; present {
				t.Errorf("metadata[%q] is present, want absent", tt.wantAbsent)
			}
		})
	}
}
