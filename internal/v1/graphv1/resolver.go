package graphv1

import (
	"fmt"

	"github.com/vektah/gqlparser/v2/ast"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/api/internal/graph"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/v1/graphv1/gengqlv1"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

type Resolver struct {
	pubsubTopic *pubsub.Topic
	log         logrus.FieldLogger
}

type ResolverOption func(*Resolver)

func WithLogger(log logrus.FieldLogger) ResolverOption {
	return func(r *Resolver) {
		r.log = log
	}
}

func NewResolver(topic *pubsub.Topic, opts ...ResolverOption) *Resolver {
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

func NewHandler(config gengqlv1.Config, log logrus.FieldLogger) (*handler.Server, error) {
	metricsMiddleware, err := graph.NewMetrics(otel.Meter("graphv1"))
	if err != nil {
		return nil, fmt.Errorf("create metrics middleware: %w", err)
	}

	schema := gengqlv1.NewExecutableSchema(config)
	graphHandler := handler.New(schema)
	graphHandler.Use(metricsMiddleware)
	graphHandler.AddTransport(graph.SSE{})
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
	return graphHandler, nil
}
