package model

import (
	"fmt"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type KafkaTopic struct {
	Name    string            `json:"name"`
	ID      scalar.Ident      `json:"id"`
	ACL     []*ACL            `json:"acl"`
	Config  *KafkaTopicConfig `json:"config"`
	Pool    string            `json:"pool"`
	Env     Env               `json:"env"`
	GQLVars KafkaTopicGQLVars `json:"-"`
}
type KafkaTopicGQLVars struct {
	TeamSlug       slug.Slug
	OwnerReference *v1.OwnerReference
}

func (KafkaTopic) IsPersistence()           {}
func (this KafkaTopic) GetName() string     { return this.Name }
func (this KafkaTopic) GetID() scalar.Ident { return this.ID }

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
		ACL: func(as []kafka_nais_io_v1.TopicACL) []*ACL {
			ret := make([]*ACL, len(as))
			for i, a := range as {
				ret[i] = &ACL{
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
	}, nil
}
