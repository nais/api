package role

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/api/internal/v1/databasev1"
	"github.com/nais/api/internal/v1/graphv1/loaderv1"
	"github.com/nais/api/internal/v1/role/rolesql"
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
	internalQuerier     *rolesql.Queries
	userRoles           *dataloadgen.Loader[uuid.UUID, *UserRoles]
	serviceAccountRoles *dataloadgen.Loader[uuid.UUID, *ServiceAccountRoles]
}

func newLoaders(dbConn *pgxpool.Pool, opts []dataloadgen.Option) *loaders {
	db := rolesql.New(dbConn)
	dataloader := &dataloader{db: db}

	return &loaders{
		internalQuerier:     db,
		userRoles:           dataloadgen.NewLoader(dataloader.listUserRoles, opts...),
		serviceAccountRoles: dataloadgen.NewLoader(dataloader.listServiceAccountRoles, opts...),
	}
}

func db(ctx context.Context) *rolesql.Queries {
	l := fromContext(ctx)

	if tx := databasev1.TransactionFromContext(ctx); tx != nil {
		return l.internalQuerier.WithTx(tx)
	}

	return l.internalQuerier
}

type dataloader struct {
	db *rolesql.Queries
}

func (l dataloader) listUserRoles(ctx context.Context, userIDs []uuid.UUID) ([]*UserRoles, []error) {
	makeKey := func(obj *UserRoles) uuid.UUID { return obj.UserID }
	return loaderv1.LoadModelsWithError(ctx, userIDs, l.db.GetRolesForUsers, toUserRoles, makeKey)
}

func (l dataloader) listServiceAccountRoles(ctx context.Context, serviceAccountIDs []uuid.UUID) ([]*ServiceAccountRoles, []error) {
	makeKey := func(obj *ServiceAccountRoles) uuid.UUID { return obj.ServiceAccountID }
	return loaderv1.LoadModelsWithError(ctx, serviceAccountIDs, l.db.GetRolesForServiceAccounts, toServiceAccountRoles, makeKey)
}
