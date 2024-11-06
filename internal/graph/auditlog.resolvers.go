package graph

import (
	"context"

	"github.com/nais/api/internal/audit"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/team"
)

func (r *teamResolver) AuditEntries(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[audit.AuditEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return audit.ListForTeam(ctx, obj.Slug, page)
}
