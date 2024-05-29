package model

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	sync_states "github.com/nais/liberator/pkg/events"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type KafkaTopic struct {
	Name    string            `json:"name"`
	ID      scalar.Ident      `json:"id"`
	ACL     []*KafkaTopicACL  `json:"acl"`
	Config  *KafkaTopicConfig `json:"config"`
	Pool    string            `json:"pool"`
	Env     Env               `json:"env"`
	Status  *KafkaTopicStatus `json:"status"`
	GQLVars KafkaTopicGQLVars `json:"-"`
}
type KafkaTopicGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (KafkaTopic) IsPersistence()        {}
func (t KafkaTopic) GetName() string     { return t.Name }
func (t KafkaTopic) GetID() scalar.Ident { return t.ID }

func ToKafkaTopic(u *unstructured.Unstructured, env string) (*KafkaTopic, error) {
	kt := &kafka_nais_io_v1.Topic{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, kt); err != nil {
		return nil, fmt.Errorf("converting to KafkaTopic: %w", err)
	}

	teamSlug := kt.GetNamespace()

	return &KafkaTopic{
		ID:   scalar.KafkaTopicIdent("kafkatopic_" + env + "_" + teamSlug + "_" + kt.GetName()),
		Name: kt.Name,
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
					Access:      a.Access,
					Application: a.Application,
					Team:        slug.Slug(a.Team),
				}
			}
			return ret
		}(kt.Spec.ACL),
		Env: Env{
			Name: env,
			Team: teamSlug,
		},
		GQLVars: KafkaTopicGQLVars{
			TeamSlug:       slug.Slug(teamSlug),
			OwnerReference: OwnerReference(kt.OwnerReferences),
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
	}, nil
}
