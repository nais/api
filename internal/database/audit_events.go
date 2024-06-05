package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/slug"
)

type AuditEventsRepo interface {
	CreateAuditEvent(ctx context.Context, team slug.Slug, actor authz.AuthenticatedUser, action, resourceName, resourceType string, data map[string]string) error
}

var _ AuditEventsRepo = (*database)(nil)

type AuditEvent struct {
	*gensql.AuditEvent
}

func (d *database) CreateAuditEvent(ctx context.Context, team slug.Slug, actor authz.AuthenticatedUser, action, resourceName, resourceType string, data map[string]string) error {
	id, err := d.querier.CreateAuditEvent(ctx, gensql.CreateAuditEventParams{
		Team:         &team,
		Action:       action,
		Actor:        actor.Identity(),
		ResourceName: resourceName,
		ResourceType: resourceType,
	})
	if err != nil {
		return err
	}

	if data != nil {
		if err := d.createAuditEventData(ctx, id, data); err != nil {
			return err
		}
	}

	return nil
}

func (d *database) createAuditEventData(ctx context.Context, eventID uuid.UUID, data map[string]string) error {
	for key, value := range data {
		if err := d.querier.CreateAuditEventData(ctx, gensql.CreateAuditEventDataParams{
			EventID: eventID,
			Key:     key,
			Value:   value,
		}); err != nil {
			return err
		}
	}

	return nil
}
