package podlog

import (
	"context"
)

func LogStream(ctx context.Context, filter *WorkloadLogSubscriptionFilter) (<-chan *WorkloadLogLine, error) {
	if err := filter.Validate(ctx); err != nil {
		return nil, err
	}

	return fromContext(ctx).streamer.Logs(ctx, filter)
}
