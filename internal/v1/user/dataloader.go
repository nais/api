package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/user/usersql"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool, defaultOpts []dataloadgen.Option) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn, defaultOpts))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier *usersql.Queries
	userLoader      *dataloadgen.Loader[uuid.UUID, *User]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := usersql.New(dbConn)
	userLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier: db,
		userLoader:      dataloadgen.NewLoader(userLoader.list, opts...),
	}
}

type dataloader struct {
	db *usersql.Queries
}

func (l dataloader) list(ctx context.Context, userIDs []uuid.UUID) ([]*User, []error) {
	makeKey := func(obj *User) uuid.UUID { return obj.UUID }
	return loaderv1.LoadModels(ctx, userIDs, l.db.GetByIDs, toGraphUser, makeKey)
}

func db(ctx context.Context) *usersql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}