package loki

import (
	"testing"

	"github.com/nais/api/internal/environmentmapper"
)

func TestInjectEnvToQuery(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		env     string
		mapping environmentmapper.EnvironmentMapping
		want    string
		wantErr bool
	}{
		{
			name:  "simple stream selector",
			query: `{service_name="myapp"}`,
			env:   "dev-gcp",
			want:  `{service_name="myapp", k8s_cluster_name="dev-gcp"}`,
		},
		{
			name:  "stream selector with pipeline",
			query: `{service_name="myapp"} | json | level="error"`,
			env:   "dev-gcp",
			want:  `{service_name="myapp", k8s_cluster_name="dev-gcp"} | json | level="error"`,
		},
		{
			name:  "stream selector with line filter",
			query: `{service_name="myapp"} |= "boom"`,
			env:   "prod-gcp",
			want:  `{service_name="myapp", k8s_cluster_name="prod-gcp"} |= "boom"`,
		},
		{
			name:  "metric query",
			query: `sum(rate({service_name="myapp"}[5m]))`,
			env:   "dev-gcp",
			want:  `sum(rate({service_name="myapp", k8s_cluster_name="dev-gcp"}[5m]))`,
		},
		{
			name:  "label already present is left untouched",
			query: `{service_name="myapp", k8s_cluster_name="other"}`,
			env:   "dev-gcp",
			want:  `{service_name="myapp", k8s_cluster_name="other"}`,
		},
		{
			name:    "environment name is mapped to cluster name",
			query:   `{service_name="myapp"}`,
			env:     "dev-gcp",
			mapping: environmentmapper.EnvironmentMapping{"dev": "dev-gcp"},
			want:    `{service_name="myapp", k8s_cluster_name="dev"}`,
		},
		{
			name:    "invalid query returns error",
			query:   `not a valid logql query`,
			env:     "dev-gcp",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			environmentmapper.SetMapping(tt.mapping)
			defer environmentmapper.SetMapping(nil)

			got, err := injectEnvLabel(tt.query, tt.env)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("expected query %q, got %q", tt.want, got)
			}
		})
	}
}
