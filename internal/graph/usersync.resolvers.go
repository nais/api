package graph

import (
	"context"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/usersync"
)

func (r *queryResolver) UserSyncLog(ctx context.Context, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[usersync.UserSyncLogEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return usersync.ListLogEntries(ctx, page)
}
