package team

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamsql"
	"github.com/vikstrous/dataloadgen"
	corev1 "k8s.io/api/core/v1"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool, namespaceWatcher *watcher.Watcher[*corev1.Namespace]) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn, namespaceWatcher))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier       *teamsql.Queries
	teamLoader            *dataloadgen.Loader[slug.Slug, *Team]
	teamEnvironmentLoader *dataloadgen.Loader[envSlugName, *TeamEnvironment]
	namespaceWatcher      *watcher.Watcher[*corev1.Namespace]
}

func newLoaders(dbConn *pgxpool.Pool, namespaceWatcher *watcher.Watcher[*corev1.Namespace]) *loaders {
	db := teamsql.New(dbConn)
	teamLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier:       db,
		teamLoader:            dataloadgen.NewLoader(teamLoader.list, loader.DefaultDataLoaderOptions...),
		teamEnvironmentLoader: dataloadgen.NewLoader(teamLoader.getEnvironments, loader.DefaultDataLoaderOptions...),
		namespaceWatcher:      namespaceWatcher,
	}
}

type dataloader struct {
	db teamsql.Querier
}

func NewNamespaceWatcher(ctx context.Context, mgr *watcher.Manager) *watcher.Watcher[*corev1.Namespace] {
	w := watcher.Watch(mgr, &corev1.Namespace{})
	w.Start(ctx)
	return w
}

func (l dataloader) list(ctx context.Context, slugs []slug.Slug) ([]*Team, []error) {
	makeKey := func(obj *Team) slug.Slug { return obj.Slug }
	return loader.LoadModels(ctx, slugs, l.db.ListBySlugs, toGraphTeam, makeKey)
}

func (l dataloader) getEnvironments(ctx context.Context, ids []envSlugName) ([]*TeamEnvironment, []error) {
	makeKey := func(e *TeamEnvironment) envSlugName {
		return envSlugName{Slug: e.TeamSlug, EnvName: e.Name}
	}

	return loader.LoadModels(ctx, ids, l.getTeamEnvironmentsBySlugsAndEnvNames, toGraphTeamEnvironment, makeKey)
}

func (l dataloader) getTeamEnvironmentsBySlugsAndEnvNames(ctx context.Context, lookup []envSlugName) ([]*teamsql.TeamAllEnvironment, error) {
	slugs := make([]slug.Slug, len(lookup))
	envNames := make([]string, len(lookup))

	for i, v := range lookup {
		slugs[i] = v.Slug
		envNames[i] = v.EnvName
	}

	return l.db.ListEnvironmentsBySlugsAndEnvNames(ctx, teamsql.ListEnvironmentsBySlugsAndEnvNamesParams{
		TeamSlugs:    slugs,
		Environments: envNames,
	})
}

type envSlugName struct {
	Slug    slug.Slug
	EnvName string
}

func db(ctx context.Context) *teamsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
