package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/auditv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/team"
)

func (r *teamResolver) AuditEntries(ctx context.Context, obj *team.Team, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[auditv1.AuditEntry], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}

	return auditv1.ListForTeam(ctx, obj.Slug, page)
}
