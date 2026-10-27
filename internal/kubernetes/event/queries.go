package event

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
	"github.com/nais/api/internal/slug"
)

func ListForApplication(ctx context.Context, teamSlug slug.Slug, environment, name string, page *pagination.Pagination, filter *WorkloadEventFilter) (*WorkloadEventConnection, error) {
	return listForWorkload(ctx, teamSlug, environment, name, "Application", []string{"Application", "Deployment", "HorizontalPodAutoscaler"}, page, filter)
}

func ListForJob(ctx context.Context, teamSlug slug.Slug, environment, name string, page *pagination.Pagination, filter *WorkloadEventFilter) (*WorkloadEventConnection, error) {
	return listForWorkload(ctx, teamSlug, environment, name, "Naisjob", []string{"Naisjob", "CronJob"}, page, filter)
}

func listForWorkload(ctx context.Context, teamSlug slug.Slug, environment, name, kind string, kinds []string, page *pagination.Pagination, filter *WorkloadEventFilter) (*WorkloadEventConnection, error) {
	var from, to pgtype.Timestamptz
	if filter != nil {
		if filter.From != nil {
			from = pgtype.Timestamptz{Time: *filter.From, Valid: true}
		}
		if filter.To != nil {
			to = pgtype.Timestamptz{Time: *filter.To, Valid: true}
		}
	}
	q := fromContext(ctx).queries
	total, err := q.CountForWorkload(ctx, eventsql.CountForWorkloadParams{
		Namespace:       teamSlug.String(),
		EnvironmentName: environmentmapper.EnvironmentName(environment),
		Name:            name,
		WorkloadKind:    kind,
		Kinds:           kinds,
		From:            from,
		To:              to,
	})
	if err != nil {
		return nil, err
	}
	rows, err := q.ListForWorkload(ctx, eventsql.ListForWorkloadParams{
		Namespace:       teamSlug.String(),
		EnvironmentName: environmentmapper.EnvironmentName(environment),
		Name:            name,
		WorkloadKind:    kind,
		Kinds:           kinds,
		From:            from,
		To:              to,
		Limit:           page.Limit(),
		Offset:          page.Offset(),
	})
	if err != nil {
		return nil, err
	}
	return pagination.NewConvertConnectionWithError(rows, page, total, toGraphEvent)
}
