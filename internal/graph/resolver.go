package graph

import (
	"context"
	"fmt"
	"slices"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/unleash"
	"github.com/nais/api/internal/vulnerabilities"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/sirupsen/logrus"
	"github.com/vektah/gqlparser/v2/ast"
	"go.opentelemetry.io/otel"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type ClusterInfo struct {
	GCP bool
}

type ClusterList map[string]ClusterInfo

func (c ClusterList) GCPClusters() []string {
	if c == nil {
		return nil
	}

	var ret []string
	for cluster, info := range c {
		if info.GCP {
			ret = append(ret, cluster)
		}
	}

	return ret
}

func (c ClusterList) Names() []string {
	if c == nil {
		return nil
	}

	var ret []string
	for cluster := range c {
		ret = append(ret, cluster)
	}

	slices.SortFunc(ret, func(i, j string) int {
		if i < j {
			return -1
		}
		return 1
	})
	return ret
}

type HookdClient interface {
	Deployments(ctx context.Context, opts ...hookd.RequestOption) ([]hookd.Deploy, error)
	ChangeDeployKey(ctx context.Context, team string) (*hookd.DeployKey, error)
	DeployKey(ctx context.Context, team string) (*hookd.DeployKey, error)
}

type Resolver struct {
	hookdClient         HookdClient
	k8sClient           *k8s.Client
	vulnerabilities     *vulnerabilities.Manager
	resourceUsageClient resourceusage.ResourceUsageClient
	log                 logrus.FieldLogger
	clusters            ClusterList
	database            database.Database
	tenant              string
	tenantDomain        string
	pubsubTopic         *pubsub.Topic
	unleashMgr          *unleash.Manager
}

// NewResolver creates a new GraphQL resolver with the given dependencies
func NewResolver(
	hookdClient HookdClient,
	k8sClient *k8s.Client,
	vulnerabilitiesMgr *vulnerabilities.Manager,
	resourceUsageClient resourceusage.ResourceUsageClient,
	db database.Database,
	tenant string,
	tenantDomain string,
	clusters ClusterList,
	pubsubTopic *pubsub.Topic,
	log logrus.FieldLogger,
	unleashMgr *unleash.Manager,
) *Resolver {
	return &Resolver{
		hookdClient:         hookdClient,
		k8sClient:           k8sClient,
		vulnerabilities:     vulnerabilitiesMgr,
		resourceUsageClient: resourceUsageClient,
		tenant:              tenant,
		tenantDomain:        tenantDomain,
		log:                 log,
		database:            db,
		clusters:            clusters,
		pubsubTopic:         pubsubTopic,
		unleashMgr:          unleashMgr,
	}
}

// NewHandler creates and returns a new GraphQL handler with the given configuration
func NewHandler(config gengql.Config, log logrus.FieldLogger) (*handler.Server, error) {
	meter := otel.Meter("graph")
	metricsMiddleware, err := NewMetrics(meter)
	if err != nil {
		return nil, fmt.Errorf("create metrics middleware: %w", err)
	}

	schema := gengql.NewExecutableSchema(config)
	graphHandler := handler.New(schema)
	graphHandler.Use(metricsMiddleware)
	graphHandler.AddTransport(SSE{}) // Support subscriptions
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
