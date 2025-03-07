package redis

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type (
	RedisInstanceConnection       = pagination.Connection[*RedisInstance]
	RedisInstanceEdge             = pagination.Edge[*RedisInstance]
	RedisInstanceAccessConnection = pagination.Connection[*RedisInstanceAccess]
	RedisInstanceAccessEdge       = pagination.Edge[*RedisInstanceAccess]
)

type RedisInstance struct {
	Name              string               `json:"name"`
	Status            *RedisInstanceStatus `json:"status"`
	TeamSlug          slug.Slug            `json:"-"`
	EnvironmentName   string               `json:"-"`
	WorkloadReference *workload.Reference  `json:"-"`
}

func (RedisInstance) IsPersistence() {}
func (RedisInstance) IsSearchNode()  {}
func (RedisInstance) IsNode()        {}

func (r *RedisInstance) GetName() string { return r.Name }

func (r *RedisInstance) GetNamespace() string { return r.TeamSlug.String() }

func (r *RedisInstance) GetLabels() map[string]string { return nil }

func (r *RedisInstance) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (r *RedisInstance) DeepCopyObject() runtime.Object {
	return r
}

func (r RedisInstance) ID() ident.Ident {
	return newIdent(r.TeamSlug, r.EnvironmentName, r.Name)
}

type RedisInstanceAccess struct {
	Access            string              `json:"access"`
	TeamSlug          slug.Slug           `json:"-"`
	EnvironmentName   string              `json:"-"`
	WorkloadReference *workload.Reference `json:"-"`
}

type RedisInstanceStatus struct {
	State      string             `json:"state"`
	Conditions []metav1.Condition `json:"conditions"`
}

type RedisInstanceOrder struct {
	Field     RedisInstanceOrderField `json:"field"`
	Direction model.OrderDirection    `json:"direction"`
}

type RedisInstanceOrderField string

func (e RedisInstanceOrderField) IsValid() bool {
	return SortFilterRedisInstance.SupportsSort(e)
}

func (e RedisInstanceOrderField) String() string {
	return string(e)
}

func (e *RedisInstanceOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = RedisInstanceOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid RedisInstanceOrderField", str)
	}
	return nil
}

func (e RedisInstanceOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type RedisInstanceAccessOrder struct {
	Field     RedisInstanceAccessOrderField `json:"field"`
	Direction model.OrderDirection          `json:"direction"`
}

type RedisInstanceAccessOrderField string

func (e RedisInstanceAccessOrderField) IsValid() bool {
	return SortFilterRedisInstanceAccess.SupportsSort(e)
}

func (e RedisInstanceAccessOrderField) String() string {
	return string(e)
}

func (e *RedisInstanceAccessOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = RedisInstanceAccessOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid RedisInstanceAccessOrderField", str)
	}
	return nil
}

func (e RedisInstanceAccessOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toRedisInstance(u *unstructured.Unstructured, envName string) (*RedisInstance, error) {
	obj := &aiven_io_v1alpha1.Redis{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to Redis instance: %w", err)
	}

	return &RedisInstance{
		Name:            obj.Name,
		EnvironmentName: envName,
		Status: &RedisInstanceStatus{
			Conditions: obj.Status.Conditions,
			State:      obj.Status.State,
		},
		TeamSlug:          slug.Slug(obj.GetNamespace()),
		WorkloadReference: workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
	}, nil
}

type TeamInventoryCountRedisInstances struct {
	Total int
}
