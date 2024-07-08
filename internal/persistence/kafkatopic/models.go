package kafkatopic

import (
	"fmt"
	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/slug"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"io"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"strconv"
)

type (
	KafkaTopicConnection = pagination.Connection[*KafkaTopic]
	KafkaTopicEdge       = pagination.Edge[*KafkaTopic]
)

type KafkaTopic struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
}

func (KafkaTopic) IsPersistence() {}

func (KafkaTopic) IsNode() {}

func (k KafkaTopic) ID() ident.Ident {
	return newIdent(k.TeamSlug, k.EnvironmentName, k.Name)
}

type KafkaTopicOrder struct {
	Field     KafkaTopicOrderField   `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type KafkaTopicOrderField string

const (
	KafkaTopicOrderFieldName        KafkaTopicOrderField = "NAME"
	KafkaTopicOrderFieldEnvironment KafkaTopicOrderField = "ENVIRONMENT"
)

func (e KafkaTopicOrderField) IsValid() bool {
	switch e {
	case KafkaTopicOrderFieldName, KafkaTopicOrderFieldEnvironment:
		return true
	}
	return false
}

func (e KafkaTopicOrderField) String() string {
	return string(e)
}

func (e *KafkaTopicOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = KafkaTopicOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid KafkaTopicOrderField", str)
	}
	return nil
}

func (e KafkaTopicOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toKafkaTopic(u *unstructured.Unstructured, envName string) (*KafkaTopic, error) {
	kt := &kafka_nais_io_v1.Topic{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, kt); err != nil {
		return nil, fmt.Errorf("converting to KafkaTopic: %w", err)
	}

	teamSlug := kt.GetNamespace()

	return &KafkaTopic{
		Name:            kt.Name,
		EnvironmentName: envName,
		TeamSlug:        slug.Slug(teamSlug),
		/*
			Config: func(cfg *kafka_nais_io_v1.Config) *KafkaTopicConfig {
				if cfg == nil {
					return nil
				}
				return &KafkaTopicConfig{
					CleanupPolicy:         cfg.CleanupPolicy,
					MaxMessageBytes:       cfg.MaxMessageBytes,
					MinimumInSyncReplicas: cfg.MinimumInSyncReplicas,
					Partitions:            cfg.Partitions,
					Replication:           cfg.Replication,
					RetentionBytes:        cfg.RetentionBytes,
					RetentionHours:        cfg.RetentionHours,
					SegmentHours:          cfg.SegmentHours,
				}
			}(kt.Spec.Config),

			ACL: func(as []kafka_nais_io_v1.TopicACL) []*KafkaTopicACL {
				ret := make([]*KafkaTopicACL, len(as))
				for i, a := range as {
					ret[i] = &KafkaTopicACL{
						Access:          a.Access,
						ApplicationName: a.Application,
						TeamName:        a.Team,

						GQLVars: KafkaTopicACLGQLVars{
							Env: env,
						},
					}
				}
				return ret
			}(kt.Spec.ACL),
			Env: Env{
				Name: env,
				Team: teamSlug,
			},
			GQLVars: KafkaTopicGQLVars{
				TeamSlug: slug.Slug(teamSlug),
			},
			Status: func(s *kafka_nais_io_v1.TopicStatus) *KafkaTopicStatus {
				if s == nil {
					return nil
				}

				var syncTime, expTime, lastSyncTime *time.Time
				if t, err := time.Parse(time.RFC3339, s.SynchronizationTime); err == nil {
					syncTime = &t
				}

				if t, err := time.Parse(time.RFC3339, s.CredentialsExpiryTime); err == nil {
					expTime = &t
				}

				if t, err := time.Parse(time.RFC3339, s.LatestAivenSyncFailure); err == nil {
					lastSyncTime = &t
				}

				state := StateUnknown
				switch s.SynchronizationState {
				case sync_states.FailedPrepare,
					sync_states.FailedGenerate,
					sync_states.FailedSynchronization,
					sync_states.FailedStatusUpdate,
					sync_states.Retrying:
					state = StateNotnais
				case sync_states.Synchronized,
					sync_states.RolloutComplete:
					state = StateNais
				}

				return &KafkaTopicStatus{
					FullyQualifiedName:     s.FullyQualifiedName,
					Message:                s.Message,
					SynchronizationTime:    syncTime,
					CredentialsExpiryTime:  expTime,
					Errors:                 s.Errors,
					LatestAivenSyncFailure: lastSyncTime,
					SynchronizationState:   state,
				}
			}(kt.Status),

		*/
	}, nil
}
