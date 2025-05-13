package logging

import (
	"context"
	"strings"
	"testing"

	"github.com/nais/api/internal/workload"
)

func TestLogDestinationLoki_GrafanaURL(t *testing.T) {
	ctx := NewPackageContext(context.Background(), "test-tenant", nil)

	l := LogDestinationLoki{
		logDestinationBase: logDestinationBase{
			WorkloadType:    workload.TypeApplication,
			TeamSlug:        "my-team",
			EnvironmentName: "dev",
			WorkloadName:    "my-app",
		},
	}

	url := l.GrafanaURL(ctx)

	expectedPrefix := "https://grafana.test-tenant.cloud.nais.io/a/grafana-lokiexplore-app/explore/service/my-app/logs?"
	if len(url) < len(expectedPrefix) || url[:len(expectedPrefix)] != expectedPrefix {
		t.Errorf("URL prefix mismatch. Got: %s, want prefix: %s", url, expectedPrefix)
	}

	for _, param := range []string{
		"var-ds%3Ddev-loki",
		"var-filters%3Dservice_name%7C%3D%7Cmy-app",
		"var-filters%3Dservice_namespace%7C%3D%7Cmy-team",
	} {
		if !strings.Contains(url, param) {
			t.Errorf("URL missing expected query parameter: %s\nFull URL: %s", param, url)
		}
	}
}
