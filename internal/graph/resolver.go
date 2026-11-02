package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type resolver struct {
	tenantName  string
	pubsubTopic PubsubTopic
	log         logrus.FieldLogger
}

type Resolver struct {
	*resolver
}

type ResolverOption func(*Resolver)

func WithLogger(log logrus.FieldLogger) ResolverOption {
	return func(r *Resolver) {
		r.log = log
	}
}

func NewResolver(tenantName string, topic PubsubTopic, opts ...ResolverOption) *Resolver {
	r := &Resolver{
		resolver: &resolver{
			tenantName:  tenantName,
			pubsubTopic: topic,
		},
	}

	for _, opt := range opts {
		opt(r)
	}

	if r.log == nil {
		r.log = logrus.StandardLogger()
	}

	return r
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

	schema := newExecutableSchemaWithComplexity(gengql.NewExecutableSchema(config))
	graphHandler := handler.New(schema)
	graphHandler.Use(metricsMiddleware)
	graphHandler.AddTransport(SSE{KeepAlivePingInterval: 10 * time.Second})
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

func (t deprecationTracer) InterceptField(ctx context.Context, next graphql.Resolver) (res any, err error) {
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

	t.stats.Add(ctx, 1, metric.WithAttributes(attribute.String("field", name)))
}

func (t deprecationTracer) ExtensionName() string {
	return "DeprecatedFieldUsages"
}

func (t deprecationTracer) Validate(schema graphql.ExecutableSchema) error {
	for _, typeDef := range schema.Schema().Types {
		if typeDef.BuiltIn {
			continue
		}
		for _, field := range typeDef.Fields {
			if field.Directives.ForName("deprecated") == nil {
				continue
			}
			name := typeDef.Name + "/" + field.Name
			t.log.WithField("field", name).Debug("registering deprecated field in metrics")
			t.stats.Add(context.Background(), 0, metric.WithAttributes(attribute.String("field", name)))
		}
	}
	return nil
}

type executableSchemaWithComplexity struct {
	graphql.ExecutableSchema
	paginatedFields map[string]struct{}
}

func newExecutableSchemaWithComplexity(schema graphql.ExecutableSchema) graphql.ExecutableSchema {
	paginated := make(map[string]struct{})
	for _, typeDef := range schema.Schema().Types {
		if typeDef.BuiltIn || typeDef.Kind != ast.Object {
			continue
		}
		for _, field := range typeDef.Fields {
			if field.Arguments.ForName("first") != nil && field.Arguments.ForName("last") != nil {
				paginated[typeDef.Name+"."+field.Name] = struct{}{}
			}
		}
	}
	return &executableSchemaWithComplexity{
		ExecutableSchema: schema,
		paginatedFields:  paginated,
	}
}

func (e *executableSchemaWithComplexity) Complexity(ctx context.Context, typeName, field string, childComplexity int, rawArgs map[string]any) (int, bool) {
	if _, ok := e.paginatedFields[typeName+"."+field]; !ok {
		return 0, false
	}

	var first, last *int
	if v, ok := rawArgs["first"]; ok && v != nil {
		val, err := graphql.UnmarshalInt(v)
		if err != nil {
			return 0, false
		}
		first = &val
	}
	if v, ok := rawArgs["last"]; ok && v != nil {
		val, err := graphql.UnmarshalInt(v)
		if err != nil {
			return 0, false
		}
		last = &val
	}

	if first != nil {
		return *first * childComplexity, true
	}
	if last != nil {
		return *last * childComplexity, true
	}
	return pagination.DefaultPageSize * childComplexity, true
}
