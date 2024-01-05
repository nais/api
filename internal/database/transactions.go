package db

import "context"

func (d *database) Transaction(ctx context.Context, fn DatabaseTransactionFunc) error {
	return d.querier.Transaction(ctx, func(ctx context.Context, querier Querier) error {
		return fn(ctx, &database{querier: querier})
	})
}
