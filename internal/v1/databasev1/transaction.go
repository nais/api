package databasev1

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type ctxKey int

const (
	databaseKey ctxKey = iota
	databaseTransactionKey
)

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, databaseKey, dbConn)
}

func Transaction(ctx context.Context, callback func(ctx context.Context) error) error {
	conn := ctx.Value(databaseKey).(*pgxpool.Pool)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if err := callback(context.WithValue(ctx, databaseTransactionKey, tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func TransactionFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(databaseTransactionKey).(pgx.Tx)
	return tx
}
