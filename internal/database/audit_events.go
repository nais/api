package database

import (
	"context"
	"encoding/json"

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
	Data map[string]string
}

func (d *database) CreateAuditEvent(ctx context.Context, team slug.Slug, actor authz.AuthenticatedUser, action, resourceName, resourceType string, data map[string]string) error {
	return d.querier.Transaction(ctx, func(ctx context.Context, querier Querier) error {
		params := gensql.CreateAuditEventParams{
			Team:         &team,
			Action:       action,
			Actor:        actor.Identity(),
			ResourceName: resourceName,
			ResourceType: resourceType,
		}

		if data != nil {
			b, err := json.Marshal(data)
			if err != nil {
				return err
			}
			params.Data = b
		}

		return querier.CreateAuditEvent(ctx, params)
	})
}

func (d *database) GetAuditEventsForTeam(ctx context.Context, teamSlug slug.Slug, p Page) ([]*AuditEvent, int, error) {
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
		entries[i] = &AuditEvent{
			AuditEvent: row,
			Data:       make(map[string]string),
		}

		if row.Data != nil {
			if err := json.Unmarshal(row.Data, &entries[i].Data); err != nil {
				return nil, 0, err
			}
		}
	}

	total, err := d.querier.GetAuditEventsCountForTeam(ctx, &teamSlug)
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}
