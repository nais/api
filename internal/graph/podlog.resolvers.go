package graph

import (
	"context"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/workload/podlog"
)

func (r *subscriptionResolver) WorkloadLog(ctx context.Context, filter podlog.WorkloadLogSubscriptionFilter) (<-chan *podlog.WorkloadLogLine, error) {
	return podlog.LogStream(ctx, &filter)
}

func (r *Resolver) Subscription() gengql.SubscriptionResolver { return &subscriptionResolver{r} }

type subscriptionResolver struct{ *Resolver }
