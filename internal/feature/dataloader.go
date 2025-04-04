package feature

import (
	"context"
)

type ctxKey int

const (
	loadersKey ctxKey = iota
)

func NewLoaderContext(
	ctx context.Context,
	unleash, valkey, kafka, openSearch bool,
) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(&Features{
		Unleash: FeatureUnleash{
			Enabled: unleash,
		},
		Valkey: FeatureValkey{
			Enabled: valkey,
		},
		Kafka: FeatureKafka{
			Enabled: kafka,
		},
		OpenSearch: FeatureOpenSearch{
			Enabled: openSearch,
		},
	}))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	features *Features
}

func newLoaders(features *Features) *loaders {
	return &loaders{
		features: features,
	}
}
