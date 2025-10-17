package loki

import (
	"context"
)

func LogStream(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	if err := filter.Validate(ctx); err != nil {
		return nil, err
	}

	return fromContext(ctx).client.Tail(ctx, filter)
}
