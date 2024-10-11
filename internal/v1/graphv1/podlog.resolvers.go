package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/nais/api/internal/v1/team"
	"github.com/nais/api/internal/v1/workload/podlog"
	"k8s.io/utils/ptr"
)

func (r *subscriptionResolver) WorkloadLog(ctx context.Context, filter podlog.WorkloadLogSubscriptionFilter) (<-chan *podlog.WorkloadLogLine, error) {
	sanitized := ptr.To(filter).Sanitized()
	if err := sanitized.Validate(); err != nil {
		return nil, err
	}

	if _, err := team.Get(ctx, sanitized.Team); err != nil {
		return nil, err
	}

	return podlog.LogStream(ctx, sanitized)
}

func (r *Resolver) Subscription() gengqlv1.SubscriptionResolver { return &subscriptionResolver{r} }

type subscriptionResolver struct{ *Resolver }
