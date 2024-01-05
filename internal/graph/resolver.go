package graph

import (
	"fmt"

	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/dependencytrack"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/hookd"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/search"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	hookdClient           hookd.Client
	teamsClient           teams.Client
	k8sClient             *k8s.Client
	dependencyTrackClient *dependencytrack.Client
	resourceUsageClient   resourceusage.Client
	searcher              *search.Searcher
	log                   logrus.FieldLogger
	querier               gensql.Querier
	clusters              []string
}

// NewResolver creates a new GraphQL resolver with the given dependencies
func NewResolver(hookdClient hookd.Client, teamsClient teams.Client, k8sClient *k8s.Client, dependencyTrackClient *dependencytrack.Client, resourceUsageClient resourceusage.Client, querier gensql.Querier, clusters []string, log logrus.FieldLogger) *Resolver {
	return &Resolver{
		hookdClient:           hookdClient,
		teamsClient:           teamsClient,
		k8sClient:             k8sClient,
		dependencyTrackClient: dependencyTrackClient,
		resourceUsageClient:   resourceUsageClient,
		searcher:              search.New(teamsClient, k8sClient),
		log:                   log,
		querier:               querier,
		clusters:              clusters,
	}
}

// NewHandler creates and returns a new GraphQL handler with the given configuration
func NewHandler(config Config, meter metric.Meter, log logrus.FieldLogger) (*handler.Server, error) {
	metricsMiddleware, err := NewMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("create metrics middleware: %w", err)
	}

	schema := NewExecutableSchema(config)
	graphHandler := handler.New(schema)
	graphHandler.Use(metricsMiddleware)
	graphHandler.AddTransport(transport.SSE{}) // Support subscriptions
	graphHandler.AddTransport(transport.Options{})
	graphHandler.AddTransport(transport.POST{})
	graphHandler.SetQueryCache(lru.New(1000))
	graphHandler.Use(extension.Introspection{})
	graphHandler.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
	})
	graphHandler.SetErrorPresenter(apierror.GetErrorPresenter(log))
	return graphHandler, nil
}
