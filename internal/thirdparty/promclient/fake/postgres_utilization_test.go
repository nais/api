package fake

import (
	"context"
	"testing"
)

func TestPostgresUtilizationWithFakeMetrics(t *testing.T) {
	client := NewFakeClient(nil, nil)
	queries := []struct {
		query string
		want  float64
	}{
		{`sum(rate(container_cpu_usage_seconds_total{namespace="devteam",pod=~"pg-postgres-1-main-[0-9]+",container="postgres",image!=""}[5m])) / clamp_min(sum(kube_pod_container_resource_requests{namespace="devteam",pod=~"pg-postgres-1-main-[0-9]+",container="postgres",resource="cpu",unit="core"}), 0.001)`, 0.19},
		{`sum(container_memory_working_set_bytes{namespace="devteam",pod=~"pg-postgres-1-main-[0-9]+",container="postgres",image!=""}) / clamp_min(sum(kube_pod_container_resource_requests{namespace="devteam",pod=~"pg-postgres-1-main-[0-9]+",container="postgres",resource="memory",unit="byte"}), 1)`, 0.43},
		{`sum(kubelet_volume_stats_used_bytes{namespace="devteam",persistentvolumeclaim=~"pg-postgres-1-main-[0-9]+"}) / clamp_min(sum(kubelet_volume_stats_capacity_bytes{namespace="devteam",persistentvolumeclaim=~"pg-postgres-1-main-[0-9]+"}), 1)`, 0.27},
	}
	for _, tt := range queries {
		values, err := client.Query(context.Background(), "dev", tt.query)
		if err != nil || len(values) != 1 || float64(values[0].Value) != tt.want {
			t.Errorf("query %q: got %v, %v; want %v", tt.query, values, err, tt.want)
		}
	}
}
