package dataloader

import (
	"context"
	"net/http"

	"github.com/graph-gophers/dataloader/v7"
	"github.com/nais/api/internal/db"
	"github.com/nais/api/internal/metrics"
)

type ctxKey string

const loadersKey = ctxKey("dataloaders")

// Loaders wrap your data loaders to inject via middleware
type Loaders struct {
	UsersLoader     *dataloader.Loader[string, *db.User]
	TeamsLoader     *dataloader.Loader[string, *db.Team]
	UserRolesLoader *dataloader.Loader[string, []*db.UserRole]
}

// NewLoaders instantiates data loaders for the middleware
func NewLoaders(database db.Database) *Loaders {
	// define the data loader
	usersReader := &UserReader{db: database}
	teamsReader := &TeamReader{db: database}
	userRolesReader := &UserRoleReader{db: database}

	loaders := &Loaders{
		UsersLoader: dataloader.NewBatchedLoader(usersReader.load,
			dataloader.WithCache(usersReader.newCache()),
			dataloader.WithInputCapacity[string, *db.User](5000),
		),
		TeamsLoader: dataloader.NewBatchedLoader(teamsReader.load,
			dataloader.WithCache(teamsReader.newCache()),
			dataloader.WithInputCapacity[string, *db.Team](500),
		),
		UserRolesLoader: dataloader.NewBatchedLoader(userRolesReader.load,
			dataloader.WithCache(userRolesReader.newCache()),
			dataloader.WithInputCapacity[string, []*db.UserRole](5000),
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
