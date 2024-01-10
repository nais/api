package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/nais/api/internal/auditlogger"
	db "github.com/nais/api/internal/database"
	"github.com/nais/api/internal/database/gensql"
	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/gengql"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/k8s"
	"github.com/nais/api/internal/logger"
	"github.com/nais/api/internal/resourceusage"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/thirdparty/dependencytrack"
	"github.com/nais/api/internal/thirdparty/hookd"
	"github.com/nais/api/internal/usersync"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	hookdClient           hookd.Client
	k8sClient             *k8s.Client
	dependencyTrackClient *dependencytrack.Client
	resourceUsageClient   resourceusage.Client
	searcher              *search.Searcher
	log                   logrus.FieldLogger
	querier               gensql.Querier
	clusters              []string

	// TODO(thokra) Add this to NewResolver
	teamSyncHandler interface {
		ScheduleAllTeams(ctx context.Context, correlationID uuid.UUID) ([]*db.Team, error)
		InitReconcilers(ctx context.Context) error
		UseReconciler(reconciler db.Reconciler) error
		RemoveReconciler(reconcilerName sqlc.ReconcilerName)
		SyncTeams(ctx context.Context)
		UpdateMetrics(ctx context.Context)
		DeleteTeam(teamSlug slug.Slug, correlationID uuid.UUID) error
		Close()
	}
	database        db.Database
	tenantDomain    string
	userSync        chan<- uuid.UUID
	systemName      logger.ComponentName
	auditLogger     auditlogger.AuditLogger
	gcpEnvironments []string
	userSyncRuns    *usersync.RunsHandler
}

// NewResolver creates a new GraphQL resolver with the given dependencies
func NewResolver(hookdClient hookd.Client, k8sClient *k8s.Client, dependencyTrackClient *dependencytrack.Client, resourceUsageClient resourceusage.Client, querier gensql.Querier, clusters []string, log logrus.FieldLogger) *Resolver {
	return &Resolver{
		hookdClient:           hookdClient,
		k8sClient:             k8sClient,
		dependencyTrackClient: dependencyTrackClient,
		resourceUsageClient:   resourceUsageClient,
		searcher:              search.New(querier, k8sClient),
		log:                   log,
		querier:               querier,
		clusters:              clusters,
	}
}

// NewHandler creates and returns a new GraphQL handler with the given configuration
func NewHandler(config gengql.Config, meter metric.Meter, log logrus.FieldLogger) (*handler.Server, error) {
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
// func (r *Resolver) addTeamToReconcilerQueue(input teamsync.Input) error {
// 	err := r.teamSyncHandler.Schedule(input)
// 	if err != nil {
// 		r.log.WithTeamSlug(string(input.TeamSlug)).WithError(err).Errorf("add team to reconciler queue")
// 		return apierror.Errorf("api is about to restart, unable to reconcile team: %q", input.TeamSlug)
// 	}
// 	return nil
// }

// reconcileTeam Trigger team reconcilers for a given team
func (r *Resolver) reconcileTeam(_ context.Context, correlationID uuid.UUID, slug slug.Slug) error {
	// input := teamsync.Input{
	// 	TeamSlug:      slug,
	// 	CorrelationID: correlationID,
	// }

	// return r.addTeamToReconcilerQueue(input)
	return nil
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
