package graph

import (
	"context"

	"github.com/nais/api/internal/logstreamer"
)

func (r *subscriptionResolver) Log(ctx context.Context, filter logstreamer.LogSubscriptionFilter) (<-chan *logstreamer.LogLine, error) {
	return logstreamer.LogStream(ctx, &filter)
}
