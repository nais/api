package loki

import (
	"context"
	"fmt"
)

func LogStream(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	fmt.Printf("LogStream called with filter: %+v\n", *filter.InitialBatch.Limit) // Debug print to verify filter content
	if err := filter.Validate(); err != nil {
		return nil, err
	}

	return fromContext(ctx).querier.Tail(ctx, filter)
}
