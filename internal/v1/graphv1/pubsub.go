package graphv1

import (
	"context"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

func (r *Resolver) triggerTeamCreatedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) error {
	return r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_TEAM_CREATED,
		&protoapi.EventTeamCreated{
			Slug: teamSlug.String(),
		},
		correlationID,
	)
}

func (r *Resolver) triggerTeamUpdatedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) error {
	return r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_TEAM_UPDATED,
		&protoapi.EventTeamUpdated{
			Slug: teamSlug.String(),
		},
		correlationID,
	)
}

func (r *Resolver) triggerEvent(ctx context.Context, event protoapi.EventTypes, msg proto.Message, correlationID uuid.UUID) error {
	ctx, span := otel.Tracer("").
		Start(ctx, "trigger Pub/Sub event", trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
			semconv.EventName(event.String()),
			semconv.MessagingDestinationNameKey.String(r.pubsubTopic.String()),
			semconv.MessagingSystemGCPPubsub,
		))
	defer span.End()

	b, err := proto.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		return err
	}

	attrs := map[string]string{
		"CorrelationID": correlationID.String(),
		"EventType":     event.String(),
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(attrs))

	pubsubMessage := &pubsub.Message{
		Data:       b,
		Attributes: attrs,
	}
	id, err := r.pubsubTopic.Publish(ctx, pubsubMessage).Get(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return err
	} else {
		span.SetAttributes(semconv.MessagingMessageID(id))
	}

	r.log.WithFields(logrus.Fields{
		"message":       msg,
		"correlationID": correlationID,
		"event":         event,
	}).Debugf("Published Pub/Sub message")
	return nil
}
