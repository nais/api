package database

import (
	"context"
	"encoding/json"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

type AuditEventsRepo interface {
	CreateAuditEvent(ctx context.Context, event AuditEventInput) error
	GetAuditEventsForTeam(ctx context.Context, teamSlug slug.Slug, p Page) ([]*AuditEvent, int, error)
	GetAuditEventsForTeamByResource(ctx context.Context, teamSlug slug.Slug, resourceType string, p Page) ([]*AuditEvent, int, error)
}

var _ AuditEventsRepo = (*database)(nil)

type AuditEvent struct {
	*gensql.AuditEvent
}

type AuditEventInput interface {
	GetAction() string
	GetActor() string
	GetData() any
	GetResourceType() string
	GetResourceName() string
	GetTeam() slug.Slug
}

func (d *database) CreateAuditEvent(ctx context.Context, event AuditEventInput) error {
	return d.querier.Transaction(ctx, func(ctx context.Context, querier Querier) error {
		var data []byte
		if event.GetData() != nil {
			var err error

			data, err = json.Marshal(event.GetData())
			if err != nil {
				return err
			}
		}

		return querier.CreateAuditEvent(ctx, gensql.CreateAuditEventParams{
			Team:         ptr.To(event.GetTeam()),
			Action:       event.GetAction(),
			Actor:        event.GetActor(),
			Data:         data,
			ResourceName: event.GetResourceName(),
			ResourceType: event.GetResourceType(),
		})
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
		entries[i] = &AuditEvent{AuditEvent: row}
	}

	total, err := d.querier.GetAuditEventsCountForTeam(ctx, &teamSlug)
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}

func (d *database) GetAuditEventsForTeamByResource(ctx context.Context, teamSlug slug.Slug, resourceType string, p Page) ([]*AuditEvent, int, error) {
	rows, err := d.querier.GetAuditEventsForTeamByResource(ctx, gensql.GetAuditEventsForTeamByResourceParams{
		Team:         &teamSlug,
		Offset:       int32(p.Offset),
		Limit:        int32(p.Limit),
		ResourceType: resourceType,
	})
	if err != nil {
		return nil, 0, err
	}

	entries := make([]*AuditEvent, len(rows))
	for i, row := range rows {
		entries[i] = &AuditEvent{AuditEvent: row}
	}

	total, err := d.querier.GetAuditEventsCountForTeamByResource(ctx, gensql.GetAuditEventsCountForTeamByResourceParams{
		Team:         &teamSlug,
		ResourceType: resourceType,
	})
	if err != nil {
		return nil, 0, err
	}

	return entries, int(total), nil
}
