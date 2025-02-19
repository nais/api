package logging

import (
	"context"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewPackageContext(ctx context.Context, tenantName string, defaultLogDestinations []SupportedLogDestination) context.Context {
	return context.WithValue(ctx, loadersKey, &loaders{
		tenantName:             tenantName,
		defaultLogDestinations: defaultLogDestinations,
	})
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	tenantName             string
	defaultLogDestinations []SupportedLogDestination
}
