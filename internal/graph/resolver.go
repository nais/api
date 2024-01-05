package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/gobuffalo/logger"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/db"
	"github.com/nais/api/internal/dependencytrack"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/hookd"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/sqlc"
	"github.com/nais/api/internal/types"
	"github.com/nais/api/internal/usersync"
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

	// TODO(thokra) Add this to NewResolver
	teamSyncHandler teamsync.Handler
	database        db.Database
	tenantDomain    string
	userSync        chan<- uuid.UUID
	systemName      types.ComponentName
	auditLogger     auditlogger.AuditLogger
	gcpEnvironments []string
	log             logger.Logger
	userSyncRuns    *usersync.RunsHandler
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

// GetQueriedFields Get a map of queried fields for the given context with the field names as keys
func GetQueriedFields(ctx context.Context) map[string]bool {
	fields := make(map[string]bool)
	for _, field := range graphql.CollectAllFields(ctx) {
		fields[field] = true
	}
	return fields
}

// addTeamToReconcilerQueue add a team (enclosed in an input) to the reconciler queue
func (r *Resolver) addTeamToReconcilerQueue(input teamsync.Input) error {
	err := r.teamSyncHandler.Schedule(input)
	if err != nil {
		r.log.WithTeamSlug(string(input.TeamSlug)).WithError(err).Errorf("add team to reconciler queue")
		return apierror.Errorf("api is about to restart, unable to reconcile team: %q", input.TeamSlug)
	}
	return nil
}

// reconcileTeam Trigger team reconcilers for a given team
func (r *Resolver) reconcileTeam(_ context.Context, correlationID uuid.UUID, slug slug.Slug) error {
	input := teamsync.Input{
		TeamSlug:      slug,
		CorrelationID: correlationID,
	}

	return r.addTeamToReconcilerQueue(input)
}

func (r *Resolver) getTeamBySlug(ctx context.Context, slug slug.Slug) (*db.Team, error) {
	team, err := r.database.GetTeamBySlug(ctx, slug)
	if err != nil {
		return nil, apierror.ErrTeamNotExist
	}

	return team, nil
}

func sqlcRoleFromTeamRole(teamRole model.TeamRole) (sqlc.RoleName, error) {
	switch teamRole {
	case model.TeamRoleMember:
		return sqlc.RoleNameTeammember, nil
	case model.TeamRoleOwner:
		return sqlc.RoleNameTeamowner, nil
	}

	return "", fmt.Errorf("invalid team role: %v", teamRole)
}
