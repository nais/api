package databasev1

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

// Transaction executes a callback function within a transaction. The first call to Transaction starts a new
// transaction, and subsequent calls will execute the callback function in the same transaction.
func Transaction(ctx context.Context, callback func(ctx context.Context) error) error {
	var conn interface {
		Begin(context.Context) (pgx.Tx, error)
	}

	if tx := TransactionFromContext(ctx); tx != nil {
		conn = tx
	} else {
		conn = ctx.Value(databaseKey).(*pgxpool.Pool)
	}

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

// TransactionFromContext will return a potentially open transaction from the context, nil if none exists.
func TransactionFromContext(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(databaseTransactionKey).(pgx.Tx)
	return tx
}
