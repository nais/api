package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type Resolver struct {
	pubsubTopic PubsubTopic
	log         logrus.FieldLogger
}

type ResolverOption func(*Resolver)

func WithLogger(log logrus.FieldLogger) ResolverOption {
	return func(r *Resolver) {
		r.log = log
	}
}

func NewResolver(topic PubsubTopic, opts ...ResolverOption) *Resolver {
	resolver := &Resolver{
		pubsubTopic: topic,
	}

	for _, opt := range opts {
		opt(resolver)
	}

	if resolver.log == nil {
		resolver.log = logrus.StandardLogger()
	}

	return resolver
}

func NewHandler(config gengql.Config, log logrus.FieldLogger) (*handler.Server, error) {
	metricsMiddleware, err := NewMetrics(otel.Meter("graph"))
	if err != nil {
		return nil, fmt.Errorf("create metrics middleware: %w", err)
	}

	deprecationTracer, err := newDeprecationTracer(log)
	if err != nil {
		return nil, fmt.Errorf("create deprecation tracer: %w", err)
	}

	schema := gengql.NewExecutableSchema(config)
	graphHandler := handler.New(schema)
	graphHandler.Use(metricsMiddleware)
	graphHandler.AddTransport(SSE{})
	graphHandler.AddTransport(transport.Options{})
	graphHandler.AddTransport(transport.POST{})
	graphHandler.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	graphHandler.Use(extension.Introspection{})
	graphHandler.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})
	graphHandler.SetErrorPresenter(apierror.GetErrorPresenter(log))
	graphHandler.Use(otelgqlgen.Middleware(
		otelgqlgen.WithoutVariables(),
		otelgqlgen.WithCreateSpanFromFields(func(ctx *graphql.FieldContext) bool {
			return ctx.IsResolver
		}),
	))
	graphHandler.Use(extension.FixedComplexityLimit(100_000_000))
	graphHandler.Use(deprecationTracer)

	return graphHandler, nil
}

// deprecationTracer counts usage of deprecated fields in GraphQL schema.
type deprecationTracer struct {
	// It's implemented as a graphql.HandlerExtension instead of using the
	// DirectiveRoot since gqlgen does not include `@deprecated` in the generated
	// DirectiveRoot.

	stats metric.Int64Counter
	log   logrus.FieldLogger
}

func newDeprecationTracer(log logrus.FieldLogger) (deprecationTracer, error) {
	meter := otel.GetMeterProvider().Meter("nais_api_deprecated_field_usage")
	runsCounter, err := meter.Int64Counter("nais_api_deprecated_field_usage", metric.WithDescription("Counts deprecated field usage in GraphQL"))
	if err != nil {
		return deprecationTracer{}, fmt.Errorf("create runs counter: %w", err)
	}

	return deprecationTracer{
		stats: runsCounter,
		log:   log,
	}, nil
}

func (t deprecationTracer) InterceptField(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
	fieldCtx := graphql.GetFieldContext(ctx)
	t.processField(ctx, fieldCtx)
	return next(ctx)
}

func (t deprecationTracer) processField(ctx context.Context, fieldCtx *graphql.FieldContext) {
	if fieldCtx == nil {
		return
	}
	if fieldCtx.Field.Definition == nil {
		return
	}
	if fieldCtx.Field.Definition.Directives == nil {
		return
	}
	directive := fieldCtx.Field.Definition.Directives.ForName("deprecated")
	if directive == nil {
		return
	}
	name := fieldCtx.Field.ObjectDefinition.Name + "/" + fieldCtx.Field.Name
	t.log.WithField("field", name).Debug("deprecated field used")
	t.stats.Add(ctx, 1, metric.WithAttributes(
		attribute.String("field", name),
	))
}

func (t deprecationTracer) ExtensionName() string {
	return "DeprecatedFieldUsages"
}

func (t deprecationTracer) Validate(schema graphql.ExecutableSchema) error {
	return nil
}
