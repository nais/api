package promclient

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestInjectEnvToQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{
			name:  "VectorSelector",
			query: `avg(disk_used_percent{service="valkey-nais-rss"})`,
			want:  `avg(disk_used_percent{k8s_cluster_name="fancy-dev",service="valkey-nais-rss"})`,
		},
		{
			name:  "Scalar",
			query: `100 - avg by (cpu) (cpu_usage_idle{service="valkey-nais-rss"})`,
			want:  `100 - avg by (cpu) (cpu_usage_idle{k8s_cluster_name="fancy-dev",service="valkey-nais-rss"})`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := injectEnvToQuery(tt.query, "fancy-dev")
			if gotErr != nil {
				t.Errorf("injectEnvToQuery() failed: %v", gotErr)
				return
			}

			if !cmp.Equal(tt.want, got) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}
