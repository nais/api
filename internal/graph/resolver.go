package graph

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/database/teamsearch"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/usersync"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/sirupsen/logrus"
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

type HookdClient interface {
	Deployments(ctx context.Context, opts ...hookd.RequestOption) ([]hookd.Deploy, error)
	ChangeDeployKey(ctx context.Context, team string) (*hookd.DeployKey, error)
	DeployKey(ctx context.Context, team string) (*hookd.DeployKey, error)
}

type DependencytrackClient interface {
	VulnerabilitySummary(ctx context.Context, app *dependencytrack.AppInstance) (*model.Vulnerability, error)
	GetVulnerabilities(ctx context.Context, apps []*dependencytrack.AppInstance) ([]*model.Vulnerability, error)
}

type Resolver struct {
	hookdClient           HookdClient
	k8sClient             *k8s.Client
	dependencyTrackClient DependencytrackClient
	resourceUsageClient   resourceusage.Client
	searcher              *search.Searcher
	log                   logrus.FieldLogger
	clusters              ClusterList
	database              database.Database
	tenantDomain          string
	userSync              chan<- uuid.UUID
	auditLogger           auditlogger.AuditLogger
	userSyncRuns          *usersync.RunsHandler
	pubsubTopic           *pubsub.Topic
}

// NewResolver creates a new GraphQL resolver with the given dependencies
func NewResolver(
	hookdClient HookdClient,
	k8sClient *k8s.Client,
	dependencyTrackClient DependencytrackClient,
	resourceUsageClient resourceusage.Client,
	db database.Database,
	tenantDomain string,
	userSync chan<- uuid.UUID,
	auditLogger auditlogger.AuditLogger,
	clusters ClusterList,
	userSyncRuns *usersync.RunsHandler,
	pubsubTopic *pubsub.Topic,
	log logrus.FieldLogger,
) *Resolver {
	return &Resolver{
		hookdClient:           hookdClient,
		k8sClient:             k8sClient,
		dependencyTrackClient: dependencyTrackClient,
		resourceUsageClient:   resourceUsageClient,
		tenantDomain:          tenantDomain,
		userSync:              userSync,
		auditLogger:           auditLogger,
		searcher:              search.New(teamsearch.New(db), k8sClient),
		log:                   log,
		database:              db,
		userSyncRuns:          userSyncRuns,
		clusters:              clusters,
		pubsubTopic:           pubsubTopic,
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
	graphHandler.AddTransport(transport.SSE{}) // Support subscriptions
	graphHandler.AddTransport(transport.Options{})
	graphHandler.AddTransport(transport.POST{})
	graphHandler.SetQueryCache(lru.New(1000))
	graphHandler.Use(extension.Introspection{})
	graphHandler.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
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

// GetQueriedFields Get a map of queried fields for the given context with the field names as keys
func GetQueriedFields(ctx context.Context) map[string]bool {
	fields := make(map[string]bool)
	for _, field := range graphql.CollectAllFields(ctx) {
		fields[field] = true
	}
	return fields
}

func (r *Resolver) getTeamBySlug(ctx context.Context, slug slug.Slug) (*database.Team, error) {
	team, err := r.database.GetTeamBySlug(ctx, slug)
	if err != nil {
		return nil, apierror.ErrTeamNotExist
	}

	return team, nil
}

func gensqlRoleFromTeamRole(teamRole model.TeamRole) (gensql.RoleName, error) {
	switch teamRole {
	case model.TeamRoleMember:
		return gensql.RoleNameTeammember, nil
	case model.TeamRoleOwner:
		return gensql.RoleNameTeamowner, nil
	}

	return "", fmt.Errorf("invalid team role: %v", teamRole)
}
