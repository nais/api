package graph

import (
	"context"

	"github.com/nais/api/internal/log"
)

func (r *subscriptionResolver) Log(ctx context.Context, filter log.LogSubscriptionFilter) (<-chan *log.LogLine, error) {
	return log.LogStream(ctx, &filter)
}
