package graph

import (
	"context"

	"github.com/nais/api/internal/loki"
)

func (r *subscriptionResolver) Log(ctx context.Context, filter loki.LogSubscriptionFilter) (<-chan *loki.LogLine, error) {
	return loki.LogStream(ctx, &filter)
}
