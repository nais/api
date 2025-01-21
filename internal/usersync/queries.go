package usersync

import (
	"context"

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
