package graph

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
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type PubsubTopic interface {
	Publish(ctx context.Context, msg protoreflect.ProtoMessage, attrs map[string]string) (string, error)
	String() string
}

type TopicWrapper struct {
	Topic *pubsub.Topic
}

func (t *TopicWrapper) Publish(ctx context.Context, msg protoreflect.ProtoMessage, attrs map[string]string) (string, error) {
	b, err := proto.Marshal(msg)
	if err != nil {
		return "", err
	}

	res := t.Topic.Publish(ctx, &pubsub.Message{
		Data:       b,
		Attributes: attrs,
	})
	return res.Get(ctx)
}

func (t *TopicWrapper) String() string {
	return t.Topic.String()
}

func (r *Resolver) triggerTeamCreatedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) {
	r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_TEAM_CREATED,
		&protoapi.EventTeamCreated{
			Slug: teamSlug.String(),
		},
		correlationID,
	)
}

func (r *Resolver) triggerTeamUpdatedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) {
	r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_TEAM_UPDATED,
		&protoapi.EventTeamUpdated{
			Slug: teamSlug.String(),
		},
		correlationID,
	)
}

func (r *Resolver) triggerTeamDeletedEvent(ctx context.Context, teamSlug slug.Slug, correlationID uuid.UUID) {
	r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_TEAM_DELETED,
		&protoapi.EventTeamDeleted{
			Slug: teamSlug.String(),
		},
		correlationID,
	)
}

func (r *Resolver) triggerReconcilerEnabledEvent(ctx context.Context, reconcilerName string, correlationID uuid.UUID) {
	r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_RECONCILER_ENABLED,
		&protoapi.EventReconcilerEnabled{Reconciler: reconcilerName},
		correlationID,
	)
}

func (r *Resolver) triggerReconcilerDisabledEvent(ctx context.Context, reconcilerName string, correlationID uuid.UUID) {
	r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_RECONCILER_DISABLED,
		&protoapi.EventReconcilerDisabled{Reconciler: reconcilerName},
		correlationID,
	)
}

func (r *Resolver) triggerReconcilerConfiguredEvent(ctx context.Context, reconcilerName string, correlationID uuid.UUID) {
	r.triggerEvent(
		ctx,
		protoapi.EventTypes_EVENT_RECONCILER_CONFIGURED,
		&protoapi.EventReconcilerConfigured{Reconciler: reconcilerName},
		correlationID,
	)
}

func (r *Resolver) triggerEvent(ctx context.Context, event protoapi.EventTypes, msg proto.Message, correlationID uuid.UUID) {
	ctx, span := otel.Tracer("").
		Start(ctx, "trigger Pub/Sub event", trace.WithSpanKind(trace.SpanKindProducer), trace.WithAttributes(
			// semconv.EventName(event.String()),
			semconv.MessagingDestinationNameKey.String(r.pubsubTopic.String()),
			semconv.MessagingSystemGCPPubSub,
		))
	defer span.End()

	attrs := map[string]string{
		"CorrelationID": correlationID.String(),
		"EventType":     event.String(),
	}

	otel.GetTextMapPropagator().Inject(ctx, propagation.MapCarrier(attrs))

	id, err := r.pubsubTopic.Publish(ctx, msg, attrs)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetAttributes(semconv.MessagingMessageID(id))
	}

	r.log.WithFields(logrus.Fields{
		"message":       msg,
		"correlationID": correlationID,
		"event":         event,
	}).Debugf("Published Pub/Sub message")
}
