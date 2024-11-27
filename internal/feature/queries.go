package feature

import "context"

func Get(ctx context.Context) (*Features, error) {
	return fromContext(ctx).features, nil
}
