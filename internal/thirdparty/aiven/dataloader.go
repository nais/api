package aiven

import (
	"context"

	"github.com/nais/api/internal/graph/loader"
	"github.com/sirupsen/logrus"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, projects Projects, aivenClient AivenClient, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(projects, aivenClient, log))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	projects      Projects
	versionLoader *dataloadgen.Loader[ServiceKey, *string]
}

func newLoaders(projects Projects, aivenClient AivenClient, log logrus.FieldLogger) *loaders {
	versionLoader := &versionDataloader{aivenClient: aivenClient, log: log}

	return &loaders{
		projects:      projects,
		versionLoader: dataloadgen.NewLoader(versionLoader.serviceVersions, loader.DefaultDataLoaderOptions...),
	}
}
