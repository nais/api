package kafkatopic

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	kafka_nais_io_v1 "github.com/nais/liberator/pkg/apis/kafka.nais.io/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	ACLs            []*KafkaTopicACL         `json:"-"`
	TeamSlug        slug.Slug                `json:"-"`
	EnvironmentName string                   `json:"-"`
}

func (KafkaTopic) IsPersistence() {}
func (KafkaTopic) IsSearchNode()  {}
func (KafkaTopic) IsNode()        {}

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
	Field     KafkaTopicOrderField `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type KafkaTopicOrderField string

func (e KafkaTopicOrderField) IsValid() bool {
	return SortFilterTopic.SupportsSort(e)
}

func (e KafkaTopicOrderField) String() string {
	return string(e)
}

func (e *KafkaTopicOrderField) UnmarshalGQL(v any) error {
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
	Direction model.OrderDirection    `json:"direction"`
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

type KafkaTopicACLOrderField string

func (e KafkaTopicACLOrderField) IsValid() bool {
	return SortFilterTopicACL.SupportsSort(e)
}

func (e KafkaTopicACLOrderField) String() string {
	return string(e)
}

func (e *KafkaTopicACLOrderField) UnmarshalGQL(v any) error {
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
	Team           *slug.Slug `json:"team,omitempty"`
	Workload       *string    `json:"workload,omitempty"`
	ValidWorkloads *bool      `json:"validWorkloads,omitempty"`
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
		ACLs:            toKafkaTopicACLs(obj.Spec.ACL, teamSlug, envName, obj.Name),
		TeamSlug:        teamSlug,
		EnvironmentName: envName,
	}, nil
}

type TeamInventoryCountKafkaTopics struct {
	Total int
}
