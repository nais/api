package podlog

import (
	"context"
)

func LogStream(ctx context.Context, filter *WorkloadLogSubscriptionFilter) (<-chan *WorkloadLogLine, error) {
	panic("implement")
}
