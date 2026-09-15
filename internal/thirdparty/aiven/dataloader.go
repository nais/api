package aiven

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, projects Projects) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(projects))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	projects Projects
}

func newLoaders(projects Projects) *loaders {
	return &loaders{
		projects: projects,
	}
}

// Passed by value: the loader deduplicates through a map keyed on it, which a pointer key defeats.
type DataLoaderKey struct {
	Project     string
	ServiceName string
}

type ServiceMetadataLoader struct {
	Client      AivenClient
	MetadataKey string
	Log         logrus.FieldLogger
}

func (l ServiceMetadataLoader) Load(ctx context.Context, keys []DataLoaderKey) ([]string, []error) {
	wg := pool.New().WithContext(ctx)
	rets := make([]string, len(keys))
	errs := make([]error, len(keys))

	for i, key := range keys {
		wg.Go(func(ctx context.Context) error {
			res, err := l.Client.ServiceGet(ctx, key.Project, key.ServiceName)
			if err != nil {
				errs[i] = err
				return nil
			}
			switch value := res.Metadata[l.MetadataKey].(type) {
			case nil:
			case string:
				rets[i] = value
			default:
				l.Log.Warnf("Aiven reported a non-string %s for %q", l.MetadataKey, key.ServiceName)
			}
			return nil
		})
	}

	_ = wg.Wait() // Every task returns nil; failures are recorded per key.

	return rets, errs
}
