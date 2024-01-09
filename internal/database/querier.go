package database

import (
	"context"
)

func (q *Queries) Transaction(ctx context.Context, callback QuerierTransactionFunc) error {
	tx, err := q.connPool.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	qtx := &Queries{
		Queries:  q.Queries.WithTx(tx),
		connPool: q.connPool,
	}

	if err := callback(ctx, qtx); err != nil {
		return err
	}

	return tx.Commit(ctx)
}
