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
		"?var-ds=dev-loki",
		"&var-filters=service_name|%3D|my-app",
		"&var-filters=service_namespace|%3D|my-team",
	} {
		if !strings.Contains(url, param) {
			t.Errorf("URL missing expected query parameter: %s\nFull URL: %s", param, url)
		}
	}
}
