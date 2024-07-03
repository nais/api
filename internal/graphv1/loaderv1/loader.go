package loaderv1

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/http"
)

// Middleware injects data loaders into the context
func Middleware(fn func(context.Context) context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// return a middleware that injects the loader to the request context
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Note that the loaders are being created per-request. This is important because they contain caching and batching logic that must be request-scoped.
			r = r.WithContext(fn(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}

func LoadModels[Key comparable, DBModel any, GraphModel any](
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
