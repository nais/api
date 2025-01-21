package usersync

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/usersync/usersyncsql"
)

func ListLogEntries(ctx context.Context, page *pagination.Pagination) (*UserSyncLogEntryConnection, error) {
	q := db(ctx)

	ret, err := q.ListLogEntries(ctx, usersyncsql.ListLogEntriesParams{
		Offset: page.Offset(),
		Limit:  page.Limit(),
	})
	if err != nil {
		return nil, err
	}

	total, err := q.CountLogEntries(ctx)
	if err != nil {
		return nil, err
	}

	return pagination.NewConvertConnectionWithError(ret, page, total, toGraphUserSyncLogEntry)
}

func Get(ctx context.Context, uid uuid.UUID) (UserSyncLogEntry, error) {
	return fromContext(ctx).userSyncLogLoader.Load(ctx, uid)
}

func GetByIdent(ctx context.Context, id ident.Ident) (UserSyncLogEntry, error) {
	uid, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, uid)
}
