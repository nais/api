package loaderv1

import (
	"context"
	"errors"
	"net/http"
)

var ErrObjectNotFound = errors.New("object could not be found")

// Middleware injects data loaders into the context
func Middleware(fn func(context.Context) context.Context) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		// return a middleware that injects the loader to the request context
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Note that the loaders are being created per-request. This is important because they contain caching and
			// batching logic that must be request-scoped.
			r = r.WithContext(fn(r.Context()))
			next.ServeHTTP(w, r)
		})
	}
}

func LoadModels[Key comparable, DBModel comparable, GraphModel comparable](
	ctx context.Context,
	keys []Key,
	loaderFn func(context.Context, []Key) ([]DBModel, error),
	toGraphFn func(DBModel) GraphModel,
	makeKey func(GraphModel) Key,
) ([]GraphModel, []error) {
	return LoadModelsWithError(ctx, keys, loaderFn, func(obj DBModel) (GraphModel, error) {
		return toGraphFn(obj), nil
	}, makeKey)
}

func LoadModelsWithError[Key comparable, DBModel comparable, GraphModel comparable](
	ctx context.Context,
	keys []Key,
	loaderFn func(context.Context, []Key) ([]DBModel, error),
	toGraphFn func(DBModel) (GraphModel, error),
	makeKey func(GraphModel) Key,
) ([]GraphModel, []error) {
	objs, err := loaderFn(ctx, keys)
	if err != nil {
		return nil, dupErrs(len(keys), err)
	}
	return listAndErrors(keys, toGraphList(objs, toGraphFn), makeKey)
}

func listAndErrors[K comparable, O comparable](keys []K, objs graphList[O], idfn func(obj O) K) ([]O, []error) {
	ret := make([]O, len(keys))
	errs := make([]error, len(keys))
	res := make(map[K]O)
	var nillish O
	for i, obj := range objs.List {
		if objs.Errors[i] != nil {
			errs[i] = objs.Errors[i]
			continue
		}

		if obj == nillish {
			continue
		}

		res[idfn(obj)] = obj
	}

	for i, key := range keys {
		if errs[i] != nil {
			continue
		}

		obj, ok := res[key]
		if !ok {
			errs[i] = ErrObjectNotFound
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

type graphList[GraphModel comparable] struct {
	List   []GraphModel
	Errors []error
}

func toGraphList[DBModel comparable, GraphModel comparable](objs []DBModel, fn func(obj DBModel) (GraphModel, error)) graphList[GraphModel] {
	ret := graphList[GraphModel]{
		List:   make([]GraphModel, len(objs)),
		Errors: make([]error, len(objs)),
	}
	var nillish DBModel
	for i, obj := range objs {
		if obj == nillish {
			continue
		}
		ret.List[i], ret.Errors[i] = fn(obj)
	}
	return ret
}
