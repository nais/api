package graph

import (
	"context"

	"github.com/nais/api/internal/environment"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/loki"
)

func (r *environmentResolver) Logs(ctx context.Context, obj *environment.Environment, filter loki.EnvironmentLogsFilter, first *int, after *pagination.Cursor) (*loki.LogLineConnection, error) {
	return loki.EnvironmentLogs(ctx, obj.Name, &filter, first, after)
}

func (r *subscriptionResolver) Log(ctx context.Context, filter loki.LogSubscriptionFilter) (<-chan *loki.LogLine, error) {
	return loki.LogStream(ctx, &filter)
}
