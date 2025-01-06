package graph

import (
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
)

type Resolver struct {
	pubsubTopic             PubsubTopic
	log                     logrus.FieldLogger
	replacedEnvironmentName map[string]string
}

func (r *Resolver) mappedEnvironmentName(envName string) string {
	if r.replacedEnvironmentName == nil {
		return envName
	}

	if replacement, ok := r.replacedEnvironmentName[envName]; ok {
		return replacement
	}

	return envName
}

func (r *Resolver) unmappedEnvironmentName(envName string) string {
	if r.replacedEnvironmentName == nil {
		return envName
	}

	for k, v := range r.replacedEnvironmentName {
		if v == envName {
			return k
		}
	}

	return envName
}

type ResolverOption func(*Resolver)

func WithLogger(log logrus.FieldLogger) ResolverOption {
	return func(r *Resolver) {
		r.log = log
	}
}

func WithReplacedEnvironmentName(replacedEnvironmentName map[string]string) ResolverOption {
	return func(r *Resolver) {
		r.replacedEnvironmentName = replacedEnvironmentName
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

	return graphHandler, nil
}
