package dataloader

import (
	"context"
	"net/http"

	"github.com/graph-gophers/dataloader/v7"
	dlotel "github.com/graph-gophers/dataloader/v7/trace/otel"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/metrics"
	"go.opentelemetry.io/otel"
)

type ctxKey string

const loadersKey = ctxKey("dataloaders")

// Loaders wrap your data loaders to inject via middleware
type Loaders struct {
	UsersLoader     *dataloader.Loader[string, *model.User]
	TeamsLoader     *dataloader.Loader[string, *model.Team]
	UserRolesLoader *dataloader.Loader[string, []*database.UserRole]
}

// NewLoaders instantiates data loaders for the middleware
func NewLoaders(db database.Database) *Loaders {
	// define the data loader
	usersReader := &UserReader{db: db}
	teamsReader := &TeamReader{db: db}
	userRolesReader := &UserRoleReader{db: db}

	loaders := &Loaders{
		UsersLoader: dataloader.NewBatchedLoader(usersReader.load,
			dataloader.WithCache(usersReader.newCache()),
			dataloader.WithInputCapacity[string, *model.User](5000),
			dataloader.WithTracer(dlotel.NewTracer[string, *model.User](otel.Tracer("dataloader"))),
		),
		TeamsLoader: dataloader.NewBatchedLoader(teamsReader.load,
			dataloader.WithCache(teamsReader.newCache()),
			dataloader.WithInputCapacity[string, *model.Team](500),
			dataloader.WithTracer(dlotel.NewTracer[string, *model.Team](otel.Tracer("dataloader"))),
		),
		UserRolesLoader: dataloader.NewBatchedLoader(userRolesReader.load,
			dataloader.WithCache(userRolesReader.newCache()),
			dataloader.WithInputCapacity[string, []*database.UserRole](5000),
			dataloader.WithTracer(dlotel.NewTracer[string, []*database.UserRole](otel.Tracer("dataloader"))),
		),
	}

	return loaders
}

// Middleware injects data loaders into the context
func Middleware(loaders *Loaders) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			nextCtx := context.WithValue(r.Context(), loadersKey, loaders)
			r = r.WithContext(nextCtx)
			next.ServeHTTP(w, r)

			// clear cache after request is complete
			loaders.UsersLoader.ClearAll()
			metrics.IncDataloaderCacheClears(LoaderNameUsers)
			loaders.TeamsLoader.ClearAll()
			metrics.IncDataloaderCacheClears(LoaderNameTeams)
			loaders.UserRolesLoader.ClearAll()
			metrics.IncDataloaderCacheClears(LoaderNameUserRoles)
		})
	}
}

func For(ctx context.Context) *Loaders {
	return ctx.Value(loadersKey).(*Loaders)
}
