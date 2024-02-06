package loader

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/slug"
	"github.com/vikstrous/dataloadgen"
	"go.opentelemetry.io/otel"
)

type ctxKey int

const loadersKey ctxKey = iota

// Loaders wrap your data loaders to inject via middleware
type Loaders struct {
	UserLoader            *dataloadgen.Loader[uuid.UUID, *model.User]
	TeamLoader            *dataloadgen.Loader[slug.Slug, *model.Team]
	UserRolesLoader       *dataloadgen.Loader[uuid.UUID, []*model.Role]
	TeamEnvironmentLoader *dataloadgen.Loader[database.EnvSlugName, *model.Env]
}

// NewLoaders instantiates data loaders for the middleware
func NewLoaders(db database.Database) *Loaders {
	opts := []dataloadgen.Option{
		dataloadgen.WithWait(time.Millisecond),
		dataloadgen.WithBatchCapacity(250),
		dataloadgen.WithTracer(otel.Tracer("dataloader")),
	}

	// define the data loader
	ur := &userReader{db: db}
	tr := &teamReader{db: db}
	urr := &userRolesReader{db: db}
	ter := &teamEnvironmentReader{db: db}

	return &Loaders{
		UserLoader:            dataloadgen.NewLoader(ur.getUsers, opts...),
		TeamLoader:            dataloadgen.NewLoader(tr.getTeams, opts...),
		UserRolesLoader:       dataloadgen.NewLoader(urr.getUserRoles, opts...),
		TeamEnvironmentLoader: dataloadgen.NewLoader(ter.getEnvironments, opts...),
	}
}

func NewLoaderContext(ctx context.Context, db database.Database) context.Context {
	loaders := NewLoaders(db)
	return context.WithValue(ctx, loadersKey, loaders)
}

// Middleware injects data loaders into the context
func Middleware(db database.Database) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// return a middleware that injects the loader to the request context
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Note that the loaders are being created per-request. This is important because they contain caching and batching logic that must be request-scoped.
			r = r.WithContext(NewLoaderContext(r.Context(), db))
			next.ServeHTTP(w, r)
		})
	}
}

// For returns the dataloader for a given context
func For(ctx context.Context) *Loaders {
	return ctx.Value(loadersKey).(*Loaders)
}

func listAndErrors[K comparable, O any](keys []K, objs []O, idfn func(obj O) K) ([]O, []error) {
	ret := make([]O, len(keys))
	errs := make([]error, len(keys))
	res := make(map[K]O)
	for _, obj := range objs {
		res[idfn(obj)] = obj
	}

	for i, key := range keys {
		obj, ok := res[key]
		if !ok {
			errs[i] = pgx.ErrNoRows
			continue
		}
		ret[i] = obj
	}
	return ret, errs
}

func dupErrs(ln int, err error) []error {
	ret := make([]error, ln)
	for i := range ret {
		ret[i] = err
	}
	return ret
}

func toGraphList[DBModel any, GraphModel any](objs []DBModel, fn func(obj DBModel) GraphModel) []GraphModel {
	ret := make([]GraphModel, len(objs))
	for i, obj := range objs {
		ret[i] = fn(obj)
	}
	return ret
}

func loadModels[Key comparable, DBModel any, GraphModel any](
	ctx context.Context,
	keys []Key,
	loaderFn func(context.Context, []Key) ([]DBModel, error),
	toGraphFn func(DBModel) GraphModel,
	getID func(GraphModel) Key,
) ([]GraphModel, []error) {
	objs, err := loaderFn(ctx, keys)
	if err != nil {
		return nil, dupErrs(len(keys), err)
	}
	return listAndErrors(keys, toGraphList(objs, toGraphFn), getID)
}
