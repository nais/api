package event

import (
	"context"
	"encoding/json"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/auditv1/auditsql"
)

type AuditEventInput interface {
	GetAction() string
	GetActor() string
	GetData() any
	GetEnvironment() *string
	GetResourceType() string
	GetResourceName() string
	GetTeam() *slug.Slug
}

func CreateAuditEvent(ctx context.Context, event AuditEventInput) error {
	q := db(ctx)
	var data []byte
	if event.GetData() != nil {
		var err error

		data, err = json.Marshal(event.GetData())
		if err != nil {
			return err
		}
	}

	return q.Create(ctx, auditsql.CreateParams{
		Team:         event.GetTeam(),
		Action:       event.GetAction(),
		Actor:        event.GetActor(),
		Data:         data,
		Environment:  event.GetEnvironment(),
		ResourceName: event.GetResourceName(),
		ResourceType: event.GetResourceType(),
	})
}
