package opensearchversion

import (
	"context"

	"github.com/nais/api/internal/graph/loader"

	"github.com/sirupsen/logrus"
	"github.com/sourcegraph/conc/pool"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, openSearchVersionManager *Manager, logger logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(openSearchVersionManager, logger))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type AivenDataLoaderKey struct {
	Project     string
	ServiceName string
}

type loaders struct {
	versionLoader *dataloadgen.Loader[*AivenDataLoaderKey, string]
	log           logrus.FieldLogger
}

func newLoaders(openSearchVersionManager *Manager, logger logrus.FieldLogger) *loaders {
	versionLoader := &dataloader{openSearchVersionManager: openSearchVersionManager, log: logger}
	return &loaders{
		versionLoader: dataloadgen.NewLoader(versionLoader.getVersions, loader.DefaultDataLoaderOptions...),
		log:           logger,
	}
}

type dataloader struct {
	openSearchVersionManager *Manager
	log                      logrus.FieldLogger
}

func (l dataloader) getVersions(ctx context.Context, aivenDataLoaderKeys []*AivenDataLoaderKey) ([]string, []error) {
	wg := pool.New().WithContext(ctx)
	rets := make([]string, len(aivenDataLoaderKeys))
	errs := make([]error, len(aivenDataLoaderKeys))

	for i, pair := range aivenDataLoaderKeys {
		wg.Go(func(ctx context.Context) error {
			res, err := l.openSearchVersionManager.aivenClient.ServiceGet(ctx, pair.Project, pair.ServiceName)
			if err != nil {
				errs[i] = err
			} else {
				if res.Metadata != nil {
					if version, ok := res.Metadata["opensearch_version"]; ok {
						rets[i] = version.(string)
					}
				}
			}
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		l.log.WithError(err).Error("error waiting for dataloader")
	}

	return rets, errs
}
