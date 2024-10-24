package podlog

import (
	"context"
)

func LogStream(ctx context.Context, filter *WorkloadLogSubscriptionFilter) (<-chan *WorkloadLogLine, error) {
	return fromContext(ctx).streamer.Logs(ctx, filter)
}
