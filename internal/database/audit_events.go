package database

import (
	"context"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type AuditEventsRepo interface {
	CreateAuditEvent(ctx context.Context, team slug.Slug, actor authz.AuthenticatedUser, action, resourceName, resourceType string, data map[string]string) error
	GetAuditEventsForTeam(ctx context.Context, teamSlug slug.Slug, p Page) ([]*AuditEvent, int, error)
}

var _ AuditEventsRepo = (*database)(nil)

type AuditEvent struct {
	*gensql.AuditEvent
}

func (d *database) CreateAuditEvent(ctx context.Context, team slug.Slug, actor authz.AuthenticatedUser, action, resourceName, resourceType string, data map[string]string) error {
	return d.querier.Transaction(ctx, func(ctx context.Context, querier Querier) error {
		id, err := querier.CreateAuditEvent(ctx, gensql.CreateAuditEventParams{
			Team:         &team,
			Action:       action,
			Actor:        actor.Identity(),
			ResourceName: resourceName,
			ResourceType: resourceType,
		})
		if err != nil {
			return err
		}

		if data == nil {
			return nil
		}

		for key, value := range data {
			if err := querier.CreateAuditEventData(ctx, gensql.CreateAuditEventDataParams{
				EventID: id,
				Key:     key,
				Value:   value,
			}); err != nil {
				return err
			}
		}

		return nil
	})
}

func (d *database) GetAuditEventsForTeam(ctx context.Context, teamSlug slug.Slug, p Page) ([]*AuditEvent, int, error) {
	// TODO - join data for each event
	rows, err := d.querier.GetAuditEventsForTeam(ctx, gensql.GetAuditEventsForTeamParams{
		Team:   &teamSlug,
		Offset: int32(p.Offset),
		Limit:  int32(p.Limit),
	})
	if err != nil {
		return nil, 0, err
	}

	entries := make([]*AuditEvent, len(rows))
	for i, row := range rows {
		entries[i] = &AuditEvent{AuditEvent: row}
	}

	total, err := d.querier.GetAuditEventsCountForTeam(ctx, &teamSlug)
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}
