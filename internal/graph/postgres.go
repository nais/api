package graph

import (
	"context"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/internal/slug"
	liberatorv1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
)

func workloadPostgresConnectionForWorkload(ctx context.Context, teamSlug slug.Slug, environmentName string, uses []liberatorv1.PostgresUse, first *int, after *pagination.Cursor, last *int, before *pagination.Cursor) (*pagination.Connection[*postgres.WorkloadPostgres], error) {
	page, err := pagination.ParsePage(first, after, last, before)
	if err != nil {
		return nil, err
	}
	workloadPostgreses, err := postgres.WorkloadPostgresesForWorkload(ctx, teamSlug, environmentName, uses)
	if err != nil {
		return nil, err
	}
	return pagination.NewConnection(pagination.Slice(workloadPostgreses, page), page, len(workloadPostgreses)), nil
}
