package aivencredentials

import (
	"context"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/dynamic"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dynamicClients map[string]dynamic.Interface, log logrus.FieldLogger) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		dynamicClients: dynamicClients,
		log:            log,
	})
}

type loaders struct {
	dynamicClients map[string]dynamic.Interface
	log            logrus.FieldLogger
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}
