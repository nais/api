package loki_test

import (
	"testing"

	"github.com/nais/api/internal/loki"
)

func TestDefaultLokiUrlGenerator(t *testing.T) {
	tests := []struct {
		name         string
		clusterName  string
		tenant       string
		expectedHost string
	}{
		{
			name:         "non-nav tenant without -fss",
			clusterName:  "prod",
			tenant:       "tenant",
			expectedHost: "loki.prod.tenant.cloud.nais.io",
		},
		{
			name:         "non-nav tenant with -fss",
			clusterName:  "prod-fss",
			tenant:       "tenant",
			expectedHost: "loki.prod-fss.tenant.cloud.nais.io",
		},
		{
			name:         "nav tenant without -fss",
			clusterName:  "prod",
			tenant:       "nav",
			expectedHost: "loki.prod.nav.cloud.nais.io",
		},
		{
			name:         "nav tenant with -fss",
			clusterName:  "prod-fss",
			tenant:       "nav",
			expectedHost: "loki.prod.nav.cloud.nais.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := loki.DefaultLokiUrlGenerator(tt.clusterName, tt.tenant)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if url.Host != tt.expectedHost {
				t.Errorf("expected host %q, got %q", tt.expectedHost, url.Host)
			}
		})
	}
}
