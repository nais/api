package loki_test

import (
	"testing"

	"github.com/nais/api/internal/loki"
)

func TestDefaultLokiUrlGenerator(t *testing.T) {
	url, err := loki.DefaultLokiUrlGenerator("tenant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedHost := "loki.tenant.cloud.nais.io"
	if url.Host != expectedHost {
		t.Errorf("expected host %q, got %q", expectedHost, url.Host)
	}
}
