package database

import (
	"context"
	"k8s.io/utils/ptr"

	"github.com/nais/api/internal/auditevent"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type AuditEventsRepo interface {
	CreateAuditEvent(ctx context.Context, event auditevent.Event) error
	GetAuditEventsForTeam(ctx context.Context, teamSlug slug.Slug, p Page) ([]auditevent.Event, int, error)
}

var _ AuditEventsRepo = (*database)(nil)

type AuditEvent struct {
	*gensql.AuditEvent
}

func (d *database) CreateAuditEvent(ctx context.Context, event auditevent.Event) error {
	return d.querier.Transaction(ctx, func(ctx context.Context, querier Querier) error {
		data, err := event.MarshalData()
		if err != nil {
			return err
		}

		return querier.CreateAuditEvent(ctx, gensql.CreateAuditEventParams{
			Team:         ptr.To(event.Team()),
			Action:       event.Action(),
			Actor:        event.Actor(),
			Data:         data,
			ResourceName: event.ResourceName(),
			ResourceType: event.ResourceType(),
		})
	})
}

func (d *database) GetAuditEventsForTeam(ctx context.Context, teamSlug slug.Slug, p Page) ([]auditevent.Event, int, error) {
	rows, err := d.querier.GetAuditEventsForTeam(ctx, gensql.GetAuditEventsForTeamParams{
		Team:   &teamSlug,
		Offset: int32(p.Offset),
		Limit:  int32(p.Limit),
	})
	if err != nil {
		return nil, 0, err
	}

	entries := make([]auditevent.Event, len(rows))
	for i, row := range rows {
		entries[i], err = auditevent.DbRowToAuditEvent(row)
		if err != nil {
			return nil, 0, err
		}
	}

	total, err := d.querier.GetAuditEventsCountForTeam(ctx, &teamSlug)
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}
