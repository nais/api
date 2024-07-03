package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/graphv1/loaderv1"
	"github.com/nais/api/internal/user/usersql"
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
	db         usersql.Querier
	userLoader *dataloadgen.Loader[uuid.UUID, *User]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := usersql.New(dbConn)
	userLoader := &dataloader{db: db}

	return &loaders{
		db:         db,
		userLoader: dataloadgen.NewLoader(userLoader.list, opts...),
	}
}

type dataloader struct {
	db usersql.Querier
}

func (l dataloader) list(ctx context.Context, userIDs []uuid.UUID) ([]*User, []error) {
	getID := func(obj *User) uuid.UUID { return obj.ID }
	return loaderv1.LoadModels(ctx, userIDs, l.db.GetByIDs, toGraphUser, getID)
}
