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
	if !strings.HasPrefix(url, expectedPrefix) {
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

func TestLogDestinationLoki_GrafanaURL_EnvNameMapping(t *testing.T) {
	ctx := NewPackageContext(context.Background(), "test-tenant", nil)

	tests := []struct {
		envName         string
		expectedEnvName string
	}{
		{"dev", "dev"},
		{"prod", "prod"},
		{"preprod-fss", "preprod-gcp"},
		{"test-fss", "test-gcp"},
		{"fss", "fss"},
		{"prod-fssx", "prod-fssx"},
	}

	for _, tt := range tests {
		l := LogDestinationLoki{
			logDestinationBase: logDestinationBase{
				WorkloadType:    workload.TypeApplication,
				TeamSlug:        "my-team",
				EnvironmentName: tt.envName,
				WorkloadName:    "my-app",
			},
		}

		url := l.GrafanaURL(ctx)

		expectedPrefix := "https://grafana.test-tenant.cloud.nais.io/a/grafana-lokiexplore-app/explore/service/my-app/logs?"
		if !strings.HasPrefix(url, expectedPrefix) {
			t.Errorf("URL prefix mismatch for envName=%q. Got: %s, want prefix: %s", tt.envName, url, expectedPrefix)
		}

		expectedDsParam := "?var-ds=" + tt.expectedEnvName + "-loki"
		if !strings.Contains(url, expectedDsParam) {
			t.Errorf("URL missing expected datasource param for envName=%q: want %q in %q", tt.envName, expectedDsParam, url)
		}

		if !strings.Contains(url, "&var-filters=service_name|%3D|my-app") {
			t.Errorf("URL missing service_name filter for envName=%q: %q", tt.envName, url)
		}
		if !strings.Contains(url, "&var-filters=service_namespace|%3D|my-team") {
			t.Errorf("URL missing service_namespace filter for envName=%q: %q", tt.envName, url)
		}
	}
}
