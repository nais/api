package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/log"
	"github.com/nais/api/internal/slug"
)

func (r *logLineResolver) Team(ctx context.Context, obj *log.LogLine) (slug.Slug, error) {
	panic(fmt.Errorf("not implemented: Team - team"))
}

func (r *logLineResolver) Environment(ctx context.Context, obj *log.LogLine) (string, error) {
	panic(fmt.Errorf("not implemented: Environment - environment"))
}

func (r *logLineResolver) Application(ctx context.Context, obj *log.LogLine) (*string, error) {
	panic(fmt.Errorf("not implemented: Application - application"))
}

func (r *subscriptionResolver) Log(ctx context.Context, filter log.LogSubscriptionFilter) (<-chan *log.LogLine, error) {
	return log.LogStream(ctx, &filter)
}

func (r *Resolver) LogLine() gengql.LogLineResolver { return &logLineResolver{r} }

type logLineResolver struct{ *Resolver }
