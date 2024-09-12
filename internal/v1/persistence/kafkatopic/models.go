package kafkatopic

import (
	"fmt"
	"io"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type (
	KafkaTopicConnection    = pagination.Connection[*KafkaTopic]
	KafkaTopicEdge          = pagination.Edge[*KafkaTopic]
	KafkaTopicACLConnection = pagination.Connection[*KafkaTopicACL]
	KafkaTopicACLEdge       = pagination.Edge[*KafkaTopicACL]
)

type KafkaTopic struct {
	Name            string                   `json:"name"`
	Pool            string                   `json:"pool"`
	Configuration   *KafkaTopicConfiguration `json:"configuration,omitempty"`
	Status          *KafkaTopicStatus        `json:"status"`
	ACLs            []*KafkaTopicACL         `json:"-"`
	TeamSlug        slug.Slug                `json:"-"`
	EnvironmentName string                   `json:"-"`
}

func (KafkaTopic) IsPersistence() {}

func (KafkaTopic) IsNode() {}

func (k KafkaTopic) ID() ident.Ident {
	return newIdent(k.TeamSlug, k.EnvironmentName, k.Name)
}

func (k *KafkaTopic) GetName() string { return k.Name }

func (k *KafkaTopic) GetNamespace() string { return k.TeamSlug.String() }

func (k *KafkaTopic) GetLabels() map[string]string { return nil }

func (k *KafkaTopic) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (k *KafkaTopic) DeepCopyObject() runtime.Object {
	return k
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

type KafkaTopicACL struct {
	// WorkloadName is the name used for the ACL rule. Can contain wildcards.
	WorkloadName string `json:"workloadName"`
	// TeamName is the name used for the ACL rule. Can contain wildcards.
	TeamName        string    `json:"teamName"`
	Access          string    `json:"access"`
	EnvironmentName string    `json:"-"`
	TopicName       string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
}

type KafkaTopicACLOrder struct {
	Field     KafkaTopicACLOrderField `json:"field"`
	Direction modelv1.OrderDirection  `json:"direction"`
}

type KafkaTopicConfiguration struct {
	CleanupPolicy         *string `json:"cleanupPolicy,omitempty"`
	MaxMessageBytes       *int    `json:"maxMessageBytes,omitempty"`
	MinimumInSyncReplicas *int    `json:"minimumInSyncReplicas,omitempty"`
	Partitions            *int    `json:"partitions,omitempty"`
	Replication           *int    `json:"replication,omitempty"`
	RetentionBytes        *int    `json:"retentionBytes,omitempty"`
	RetentionHours        *int    `json:"retentionHours,omitempty"`
	SegmentHours          *int    `json:"segmentHours,omitempty"`
}

type KafkaTopicStatus struct {
	State string `json:"state"`
}

type KafkaTopicACLOrderField string

const (
	KafkaTopicACLOrderFieldTopicName KafkaTopicACLOrderField = "TOPIC_NAME"
	KafkaTopicACLOrderFieldTeamSlug  KafkaTopicACLOrderField = "TEAM_SLUG"
	KafkaTopicACLOrderFieldConsumer  KafkaTopicACLOrderField = "CONSUMER"
	KafkaTopicACLOrderFieldAccess    KafkaTopicACLOrderField = "ACCESS"
)

func (e KafkaTopicACLOrderField) IsValid() bool {
	switch e {
	case KafkaTopicACLOrderFieldTopicName, KafkaTopicACLOrderFieldTeamSlug, KafkaTopicACLOrderFieldConsumer, KafkaTopicACLOrderFieldAccess:
		return true
	}
	return false
}

func (e KafkaTopicACLOrderField) String() string {
	return string(e)
}

func (e *KafkaTopicACLOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = KafkaTopicACLOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid KafkaTopicAclOrderField", str)
	}
	return nil
}

func (e KafkaTopicACLOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type KafkaTopicACLFilter struct {
	Team     *slug.Slug `json:"team,omitempty"`
	Workload *string    `json:"workload,omitempty"`
}

func toKafkaTopicConfiguration(cfg *kafka_nais_io_v1.Config) *KafkaTopicConfiguration {
	if cfg == nil {
		return nil
	}

	return &KafkaTopicConfiguration{
		CleanupPolicy:         cfg.CleanupPolicy,
		MaxMessageBytes:       cfg.MaxMessageBytes,
		MinimumInSyncReplicas: cfg.MinimumInSyncReplicas,
		Partitions:            cfg.Partitions,
		Replication:           cfg.Replication,
		RetentionBytes:        cfg.RetentionBytes,
		RetentionHours:        cfg.RetentionHours,
		SegmentHours:          cfg.SegmentHours,
	}
}

func toKafkaTopicACLs(acls []kafka_nais_io_v1.TopicACL, teamSlug slug.Slug, envName, topicName string) []*KafkaTopicACL {
	ret := make([]*KafkaTopicACL, len(acls))
	for i, a := range acls {
		ret[i] = &KafkaTopicACL{
			Access:          a.Access,
			WorkloadName:    a.Application,
			TeamName:        a.Team,
			EnvironmentName: envName,
			TopicName:       topicName,
			TeamSlug:        teamSlug,
		}
	}
	return ret
}

func toKafkaTopicStatus(status *kafka_nais_io_v1.TopicStatus) *KafkaTopicStatus {
	// TODO: Implement
	return &KafkaTopicStatus{}
}

func toKafkaTopic(u *unstructured.Unstructured, envName string) (*KafkaTopic, error) {
	obj := &kafka_nais_io_v1.Topic{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to KafkaTopic: %w", err)
	}

	teamSlug := slug.Slug(obj.GetNamespace())

	return &KafkaTopic{
		Name:            obj.Name,
		Pool:            obj.Spec.Pool,
		Configuration:   toKafkaTopicConfiguration(obj.Spec.Config),
		Status:          toKafkaTopicStatus(obj.Status),
		ACLs:            toKafkaTopicACLs(obj.Spec.ACL, teamSlug, envName, obj.Name),
		TeamSlug:        teamSlug,
		EnvironmentName: envName,
	}, nil
}
