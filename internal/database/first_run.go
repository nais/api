package db

import "context"

func (d *database) IsFirstRun(ctx context.Context) (bool, error) {
	return d.querier.IsFirstRun(ctx)
}

func (d *database) FirstRunComplete(ctx context.Context) error {
	return d.querier.FirstRunComplete(ctx)
}
