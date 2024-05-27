package graph

import (
	"context"
	"fmt"
	"slices"

	"github.com/nais/api/internal/opensearch"

	"github.com/nais/api/internal/bigquery"

	"github.com/nais/api/internal/unleash"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/bucket"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/database/teamsearch"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/kafka"
	"github.com/nais/api/internal/redis"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/sqlinstance"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/usersync"
	"github.com/ravilushqa/otelgqlgen"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func (r *Resolver) workload(ctx context.Context, ownerReference *v1.OwnerReference, teamSlug slug.Slug, env string) (model.Workload, error) {
	if ownerReference == nil {
		return nil, nil
	}

	switch ownerReference.Kind {
	case "Naisjob":
		job, err := r.k8sClient.NaisJob(ctx, ownerReference.Name, string(teamSlug), env)
		if err != nil {
			r.log.WithField("jobname", ownerReference.Name).WithField("team", teamSlug).Debug("unable to find job")
			return nil, nil
		}
		return job, nil
	case "Application":
		app, err := r.k8sClient.App(ctx, ownerReference.Name, string(teamSlug), env)
		if err != nil {
			r.log.WithField("appname", ownerReference.Name).WithField("team", teamSlug).Debug("unable to find app")
			return nil, nil
		}
		return app, nil
	default:
		r.log.WithField("kind", ownerReference.Kind).Warnf("Unknown owner reference kind")
	}
	return nil, nil
}

type HookdClient interface {
	Deployments(ctx context.Context, opts ...hookd.RequestOption) ([]hookd.Deploy, error)
	ChangeDeployKey(ctx context.Context, team string) (*hookd.DeployKey, error)
	DeployKey(ctx context.Context, team string) (*hookd.DeployKey, error)
}

type DependencytrackClient interface {
	VulnerabilitySummary(ctx context.Context, app *dependencytrack.AppInstance) (*model.Vulnerability, error)
	GetVulnerabilities(ctx context.Context, apps []*dependencytrack.AppInstance, filters ...dependencytrack.Filter) ([]*model.Vulnerability, error)
	/*GetFindingsForImage(ctx context.Context, app *dependencytrack.AppInstance) (*model.Image, error)*/
	GetMetadataForImageByProjectID(ctx context.Context, projectID string) (*model.Image, error)
	GetMetadataForImage(ctx context.Context, name, version string) (*model.Image, error)
	GetFindingsForImageByProjectID(ctx context.Context, projectID string, suppressed bool) ([]*model.Finding, error)
	GetMetadataForTeam(ctx context.Context, team string) ([]*model.Image, error)
	SuppressFinding(ctx context.Context, analysisState, comment, componentID, projectID, vulnerabilityID, suppressedBy string, suppress bool) (*model.SuppressFindingResult, error)
	GetAnalysisTrailForImage(ctx context.Context, componentID, projectID, vulnerabilityID string) ([]*model.AnalysisTrail, error)
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
	sqlInstanceClient     *sqlinstance.Client
	bucketClient          *bucket.Client
	redisClient           *redis.Client
	bigQueryDatasetClient *bigquery.Client
	openSearchClient      *opensearch.Client
	kafkaClient           *kafka.Client
	unleashMgr            *unleash.Manager
}

// NewResolver creates a new GraphQL resolver with the given dependencies
func NewResolver(hookdClient HookdClient,
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
	sqlInstanceClient *sqlinstance.Client,
	bucketClient *bucket.Client,
	redisClient *redis.Client,
	bigQueryDatasetClient *bigquery.Client,
	openSearchClient *opensearch.Client,
	kafkaClient *kafka.Client,
	unleashMgr *unleash.Manager,
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
		sqlInstanceClient:     sqlInstanceClient,
		bucketClient:          bucketClient,
		redisClient:           redisClient,
		bigQueryDatasetClient: bigQueryDatasetClient,
		openSearchClient:      openSearchClient,
		kafkaClient:           kafkaClient,
		unleashMgr:            unleashMgr,
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

func gensqlRoleFromTeamRole(teamRole model.TeamRole) (gensql.RoleName, error) {
	switch teamRole {
	case model.TeamRoleMember:
		return gensql.RoleNameTeammember, nil
	case model.TeamRoleOwner:
		return gensql.RoleNameTeamowner, nil
	}

	return "", fmt.Errorf("invalid team role: %v", teamRole)
}
