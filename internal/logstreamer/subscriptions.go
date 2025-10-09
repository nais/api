package logstreamer

import (
	"context"
)

func LogStream(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	lines, err := fromContext(ctx).querier.Query(ctx, filter)
	if err != nil {
		return nil, err
	}

	c := make(chan *LogLine, 1)

	go func() {
		for _, line := range lines {
			select {
			case <-ctx.Done():
				return
			case c <- line:
			}
		}
	}()
	return c, nil
}
