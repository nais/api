package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
)

func (r *subscriptionResolver) Log(ctx context.Context, input *model.LogSubscriptionInput) (<-chan *model.LogLine, error) {
	container := ""
	selector := ""
	switch {
	case input.App != nil:
		selector = "app=" + *input.App
		container = *input.App
	case input.Job != nil:
		selector = "app=" + *input.Job
		container = *input.Job
	default:
		return nil, fmt.Errorf("must specify either app or job")
	}

	return r.k8sClient.LogStream(ctx, input.Env, input.Team.String(), selector, container, input.Instances)
}

func (r *Resolver) Subscription() gengql.SubscriptionResolver { return &subscriptionResolver{r} }

type subscriptionResolver struct{ *Resolver }
