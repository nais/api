package version

import (
	"context"
)

func GetOpenSearchVersion(ctx context.Context, key AivenDataLoaderKey) (string, error) {
	return fromContext(ctx).versionLoader.Load(ctx, &key)
}
