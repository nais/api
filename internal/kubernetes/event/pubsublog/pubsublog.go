package pubsublog

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/kubernetes/event/eventsql"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/sirupsen/logrus"
)

type Subscriber struct {
	pubsubSubscription *pubsub.Subscription
	querier            eventsql.Querier
	log                logrus.FieldLogger
	mappers            map[string]watcher.KindResolver
}

func NewSubscriber(pubsubSubscription *pubsub.Subscription, querier eventsql.Querier, mappers map[string]watcher.KindResolver, log logrus.FieldLogger) (*Subscriber, error) {
	return &Subscriber{
		pubsubSubscription: pubsubSubscription,
		querier:            querier,
		log:                log,
		mappers:            mappers,
	}, nil
}

func (s *Subscriber) Start(ctx context.Context) error {
	return s.pubsubSubscription.Receive(ctx, s.receive)
}

type LogLine struct {
	ProtoPayload struct {
		AuthenticationInfo struct {
			PrincipalEmail string `json:"principalEmail"`
		} `json:"authenticationInfo"`
		MethodName   string `json:"methodName"`
		ResourceName string `json:"resourceName"`
	} `json:"protoPayload"`

	Operation struct {
		ID uuid.UUID `json:"id"`
	} `json:"operation"`

	Resource struct {
		Labels struct {
			ClusterName string `json:"cluster_name"`
		} `json:"labels"`
	} `json:"resource"`
	Timestamp time.Time `json:"timestamp"`
}

func (s *Subscriber) receive(ctx context.Context, msg *pubsub.Message) {
	log := s.log.WithField("id", msg.ID)

	line, err := parseMsg(msg)
	if err != nil {
		log.WithError(err).Error("failed to parse log line")
		msg.Nack()
		return
	}

	res, err := parseResourceName(line.ProtoPayload.ResourceName)
	if err != nil {
		log.WithError(err).WithField("resourceName", line.ProtoPayload.ResourceName).
			Error("failed to parse resource name")
		msg.Nack()
		return
	}

	if res.Empty() {
		log.WithField("resourceName", line.ProtoPayload.ResourceName).Warn("empty resource name, skipping")
		msg.Ack()
		return
	}

	log = log.WithField("decoder", res.GVR.Group+"/"+res.GVR.Resource)

	cluster := clusterName(line.Resource.Labels.ClusterName)
	if cluster == "" {
		log.WithField("resourceName", line.ProtoPayload.ResourceName).Warn("empty cluster name, skipping")
		msg.Ack()
		return
	}

	log = log.WithField("cluster", cluster)

	decoder := decoders[res.GVR.Group+"/"+res.GVR.Resource]
	if decoder == nil {
		log.WithField("resourceName", line.ProtoPayload.ResourceName).Info("no decoder found for resource, skipping")
		decoder = parseGeneric
	}

	auditData, err := decoder(line.ProtoPayload.MethodName, *line)
	if err != nil {
		log.WithError(err).WithField("resourceName", line.ProtoPayload.ResourceName).Error("failed to decode resource")
		msg.Nack()
		return
	}

	client, ok := s.mappers[environmentmapper.ClusterName(cluster)]
	if !ok {
		log.WithField("cluster_client", environmentmapper.ClusterName(cluster)).Error("no client found for cluster")
		msg.Nack()
		return
	}

	gvks, err := client.KindsFor(*res.GVR)
	if err != nil || len(gvks) == 0 {
		log.WithError(err).WithField("resourceName", line.ProtoPayload.ResourceName).Error("failed to get kind for resource")
		msg.Nack()
		return
	}

	b, err := json.Marshal(auditData.Data)
	if err != nil {
		log.WithError(err).WithField("resourceName", line.ProtoPayload.ResourceName).Error("failed to marshal audit data")
		msg.Nack()
		return
	}

	if auditData.Reason == "" {
		auditData.Reason = "Unknown"
	}

	var upsertErr error
	s.querier.Upsert(ctx, []eventsql.UpsertParams{
		{
			EnvironmentName:   cluster,
			InvolvedKind:      gvks[0].Kind,
			InvolvedName:      res.Name,
			InvolvedNamespace: res.Namespace,
			TriggeredAt: pgtype.Timestamptz{
				Time:  line.Timestamp,
				Valid: true,
			},
			Data:   b,
			Reason: auditData.Reason,
			Uid:    line.Operation.ID,
		},
	}).Exec(func(i int, err error) {
		if err != nil {
			upsertErr = err
		}
	})

	if upsertErr != nil {
		log.WithError(upsertErr).Error("failed to upsert event")
		msg.Nack()
		return
	}

	msg.Ack()
}

func parseMsg(msg *pubsub.Message) (*LogLine, error) {
	line := &LogLine{}
	if err := json.Unmarshal(msg.Data, line); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON message data: %w", err)
	}

	return line, nil
}

func clusterName(name string) string {
	return environmentmapper.EnvironmentName(strings.TrimPrefix(name, "nais-"))
}
