package gengql

import (
	"testing"

	"github.com/nais/api/internal/loki"
)

func TestEnvironmentLogsComplexityUsesPageSize(t *testing.T) {
	logs := NewComplexityRoot().Environment.Logs
	if got := logs(1, loki.EnvironmentLogsFilter{}, nil, nil); got != loki.DefaultLogsPageSize {
		t.Errorf("expected default complexity %d, got %d", loki.DefaultLogsPageSize, got)
	}

	first := 1000
	if got := logs(1, loki.EnvironmentLogsFilter{}, &first, nil); got != first {
		t.Errorf("expected complexity %d, got %d", first, got)
	}

	if got := logs(5, loki.EnvironmentLogsFilter{}, &first, nil); got != first*5 {
		t.Errorf("expected nested complexity %d, got %d", first*5, got)
	}
}
