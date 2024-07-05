package graph

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/protoapi"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

func (r *Resolver) triggerTeamUpdatedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) {
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_TEAM_UPDATED, &protoapi.EventTeamUpdated{Slug: teamSlug.String()}, correlationID)
}

func (r *Resolver) triggerTeamDeletedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) {
	r.triggerEvent(ctx, protoapi.EventTypes_EVENT_TEAM_DELETED, &protoapi.EventTeamDeleted{Slug: teamSlug.String()}, correlationID)
}

func (r *Resolver) triggerEvent(ctx context.Context, event protoapi.EventTypes, msg proto.Message, correlationID uuid.UUID) {
	ctx, span := otel.Tracer("").Start(ctx, "trigger pubsub event", trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
		semconv.EventName(event.String()),
		semconv.MessagingDestinationNameKey.String(r.pubsubTopic.String()),
		semconv.MessagingSystemGCPPubsub,
	))
	defer span.End()

	b, err := proto.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		panic(err)
	}

	attrs := map[string]string{
		"CorrelationID": correlationID.String(),
		"EventType":     event.String(),
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(attrs))

	id, err := r.pubsubTopic.Publish(ctx, &pubsub.Message{
		Data:       b,
		Attributes: attrs,
	}).Get(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(semconv.MessagingMessageID(id))
	}
}
