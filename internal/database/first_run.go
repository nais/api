package database

import "context"

type FirstRunRepo interface {
	FirstRunComplete(ctx context.Context) error
	IsFirstRun(ctx context.Context) (bool, error)
}

func (d *database) IsFirstRun(ctx context.Context) (bool, error) {
	return d.querier.IsFirstRun(ctx)
}

func (d *database) FirstRunComplete(ctx context.Context) error {
	return d.querier.FirstRunComplete(ctx)
}
