package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/workload/podlog"
)

func (r *subscriptionResolver) WorkloadLog(ctx context.Context, filter podlog.WorkloadLogSubscriptionFilter) (<-chan *podlog.WorkloadLogLine, error) {
	return podlog.LogStream(ctx, &filter)
}

func (r *Resolver) Subscription() gengqlv1.SubscriptionResolver { return &subscriptionResolver{r} }

type subscriptionResolver struct{ *Resolver }
