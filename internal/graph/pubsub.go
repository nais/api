package graph

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"google.golang.org/protobuf/proto"
)

func (r *Resolver) triggerTeamUpdatedEvent(ctx context.Context, s slug.Slug, correlationID uuid.UUID) {
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_TEAM_UPDATED, &protoapi.EventTeamUpdated{Slug: s.String()}, correlationID)
}

func (r *Resolver) triggerEvent(ctx context.Context, event protoapi.EventTypes, msg proto.Message, correlationID uuid.UUID) {
	b, err := proto.Marshal(msg)
	if err != nil {
		panic(err)
	}

	r.pubsubTopic.Publish(ctx, &pubsub.Message{
		Data: b,
		Attributes: map[string]string{
			"CorrelationID": correlationID.String(),
			"EventType":     event.String(),
		},
	})
}
