package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	resolverTime metric.Int64Histogram
	complexity   metric.Int64Histogram
}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = &Metrics{}

func NewMetrics(meter metric.Meter) (*Metrics, error) {
	resTime, err := meter.Int64Histogram("gql_query_time", metric.WithDescription("graphql gql query time"), metric.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("failed to create gql_query_time histogram: %w", err)
	}
	complexity, err := meter.Int64Histogram("gql_query_complexity", metric.WithDescription("query complexity"), metric.WithExplicitBucketBoundaries(1000, 5000, 10000, 20000, 50000, 100000))
	if err != nil {
		return nil, fmt.Errorf("failed to create gql_query_time histogram: %w", err)
	}

	return &Metrics{
		resolverTime: resTime,
		complexity:   complexity,
	}, nil
}

func (a *Metrics) ExtensionName() string {
	return "gqlgen-metrics"
}

func (a *Metrics) Validate(_ graphql.ExecutableSchema) error {
	return nil
}

func (a *Metrics) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	return next(ctx)
}

func (a *Metrics) InterceptRootField(ctx context.Context, next graphql.RootResolver) graphql.Marshaler {
	stats := extension.GetComplexityStats(ctx)
	if stats == nil {
		return next(ctx)
	}

	a.complexity.Record(ctx, int64(stats.Complexity), metric.WithAttributes(attribute.String("operation", graphql.GetOperationContext(ctx).Operation.Name)))
	return next(ctx)
}

func (a *Metrics) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)
	if !fc.IsResolver {
		return next(ctx)
	}

	start := time.Now()
	res, err := next(ctx)
	a.resolverTime.Record(ctx, time.Since(start).Milliseconds(), metric.WithAttributes(attribute.String("resolver", fc.Field.ObjectDefinition.Name+"/"+fc.Field.Name)))
	return res, err
}
