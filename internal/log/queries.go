package log

import (
	"context"
)

func LogStream(ctx context.Context, filter *LogSubscriptionFilter) (<-chan *LogLine, error) {
	panic("produce log lines")
}
