package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
	"github.com/nais/api/internal/user/usersql"
	"github.com/vikstrous/dataloadgen"
)

type ctxKey int

const loadersKey ctxKey = iota

func NewLoaderContext(ctx context.Context, dbConn *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, loadersKey, newLoaders(dbConn))
}

func fromContext(ctx context.Context) *loaders {
	return ctx.Value(loadersKey).(*loaders)
}

type loaders struct {
	internalQuerier *usersql.Queries
	userLoader      *dataloadgen.Loader[uuid.UUID, *User]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := usersql.New(dbConn)
	userLoader := &dataloader{db: db}

	return &loaders{
		internalQuerier: db,
		userLoader:      dataloadgen.NewLoader(userLoader.list, loader.DefaultDataLoaderOptions...),
	}
}

type dataloader struct {
	db *usersql.Queries
}

func (l dataloader) list(ctx context.Context, userIDs []uuid.UUID) ([]*User, []error) {
	makeKey := func(obj *User) uuid.UUID { return obj.UUID }
	return loader.LoadModels(ctx, userIDs, l.db.GetByIDs, toGraphUser, makeKey)
}

func db(ctx context.Context) *usersql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}
