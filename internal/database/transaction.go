package database

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ctxKey int

const (
	databaseKey ctxKey = iota
	databaseTransactionKey
)

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, databaseKey, dbConn)
}

// Transaction executes a callback function within a transaction. Multiple calls to Transaction will start "nested"
// transactions (https://www.postgresql.org/docs/current/sql-savepoint.html).
func Transaction(ctx context.Context, callback func(ctx context.Context) error) error {
	tx, err := connectionFromContext(ctx).Begin(ctx)
	if err != nil {
		return err
	}

	defer tx.Rollback(ctx)

	if err := callback(context.WithValue(ctx, databaseTransactionKey, tx)); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// TransactionFromContext will return a potentially open transaction from the context, nil if none exists.
func TransactionFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(databaseTransactionKey).(pgx.Tx)
	return tx
}

type transactioner interface {
	Begin(context.Context) (pgx.Tx, error)
}

// connectionFromContext will return an open transaction if it exists, or a connection from the connection pool.
func connectionFromContext(ctx context.Context) transactioner {
	if tx := TransactionFromContext(ctx); tx != nil {
		return tx
	}

	return ctx.Value(databaseKey).(*pgxpool.Pool)
}
