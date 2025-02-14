package authz

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/auth/authz/authzsql"
	"github.com/nais/api/internal/database"
	"github.com/nais/api/internal/graph/loader"
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
	internalQuerier     *authzsql.Queries
	userRoles           *dataloadgen.Loader[uuid.UUID, *UserRoles]
	serviceAccountRoles *dataloadgen.Loader[uuid.UUID, *ServiceAccountRoles]
}

func newLoaders(dbConn *pgxpool.Pool) *loaders {
	db := authzsql.New(dbConn)
	dataloader := &dataloader{db: db}

	return &loaders{
		internalQuerier:     db,
		userRoles:           dataloadgen.NewLoader(dataloader.listUserRoles, loader.DefaultDataLoaderOptions...),
		serviceAccountRoles: dataloadgen.NewLoader(dataloader.listServiceAccountRoles, loader.DefaultDataLoaderOptions...),
	}
}

func db(ctx context.Context) *authzsql.Queries {
	l := fromContext(ctx)

	if tx := database.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}

type dataloader struct {
	db *authzsql.Queries
}

func (l dataloader) listUserRoles(ctx context.Context, userIDs []uuid.UUID) ([]*UserRoles, []error) {
	makeKey := func(obj *UserRoles) uuid.UUID { return obj.UserID }
	return loader.LoadModelsWithError(ctx, userIDs, l.db.GetRolesForUsers, toUserRoles, makeKey)
}

func (l dataloader) listServiceAccountRoles(ctx context.Context, serviceAccountIDs []uuid.UUID) ([]*ServiceAccountRoles, []error) {
	makeKey := func(obj *ServiceAccountRoles) uuid.UUID { return obj.ServiceAccountID }
	return loader.LoadModelsWithError(ctx, serviceAccountIDs, l.db.GetRolesForServiceAccounts, toServiceAccountRoles, makeKey)
}
