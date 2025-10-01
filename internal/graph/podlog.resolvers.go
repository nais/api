package graph

import (
	"context"

	"github.com/nais/api/internal/workload/podlog"
)

func (r *subscriptionResolver) WorkloadLog(ctx context.Context, filter podlog.WorkloadLogSubscriptionFilter) (<-chan *podlog.WorkloadLogLine, error) {
	return podlog.LogStream(ctx, &filter)
}
