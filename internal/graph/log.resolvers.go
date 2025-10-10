package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model/donotuse"
	"github.com/nais/api/internal/loki"
)

func (r *subscriptionResolver) Log(ctx context.Context, filter loki.LogSubscriptionFilter) (<-chan *loki.LogLine, error) {
	return loki.LogStream(ctx, &filter)
}

func (r *logSubscriptionFilterResolver) InitialBatch(ctx context.Context, obj *loki.LogSubscriptionFilter, data *donotuse.LogSubscriptionInitialBatch) error {
	panic(fmt.Errorf("not implemented: InitialBatch - initialBatch"))
}

func (r *Resolver) LogSubscriptionFilter() gengql.LogSubscriptionFilterResolver {
	return &logSubscriptionFilterResolver{r}
}

type logSubscriptionFilterResolver struct{ *Resolver }
