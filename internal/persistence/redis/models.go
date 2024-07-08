package redis

import (
	"fmt"
	"io"
	"strconv"

	"github.com/nais/api/internal/graphv1/ident"
	"github.com/nais/api/internal/graphv1/modelv1"
	"github.com/nais/api/internal/graphv1/pagination"
	"github.com/nais/api/internal/persistence"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

type (
	RedisInstanceConnection = pagination.Connection[*RedisInstance]
	RedisInstanceEdge       = pagination.Edge[*RedisInstance]
)

type RedisInstance struct {
	Name            string                 `json:"name"`
	Workload        workload.Workload      `json:"workload,omitempty"`
	Status          RedisInstanceStatus    `json:"status"`
	TeamSlug        slug.Slug              `json:"-"`
	EnvironmentName string                 `json:"-"`
	OwnerReference  *metav1.OwnerReference `json:"-"`
}

func (RedisInstance) IsPersistence() {}

func (RedisInstance) IsNode() {}

func (r RedisInstance) ID() ident.Ident {
	return newIdent(r.TeamSlug, r.EnvironmentName, r.Name)
}

type RedisInstanceAccess struct {
	Workload workload.Workload `json:"workload"`
	Role     string            `json:"role"`
}

type RedisInstanceStatus struct {
	State      string             `json:"state"`
	Conditions []metav1.Condition `json:"conditions"`
}

func toRedisInstance(u *unstructured.Unstructured, envName string) (*RedisInstance, error) {
	redis := &aiven_io_v1alpha1.Redis{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, redis); err != nil {
		return nil, fmt.Errorf("converting to Bucket: %w", err)
	}

	teamSlug := redis.GetNamespace()

	r := &RedisInstance{
		Name:            redis.Name,
		EnvironmentName: envName,
		Status: RedisInstanceStatus{
			Conditions: redis.Status.Conditions,
			State:      redis.Status.State,
		},
		TeamSlug:       slug.Slug(teamSlug),
		OwnerReference: persistence.OwnerReference(redis.OwnerReferences),
	}

	return r, nil
}

type RedisInstanceOrder struct {
	Field     RedisInstanceOrderField `json:"field"`
	Direction modelv1.OrderDirection  `json:"direction"`
}

type RedisInstanceOrderField string

const (
	RedisInstanceOrderFieldName        RedisInstanceOrderField = "NAME"
	RedisInstanceOrderFieldEnvironment RedisInstanceOrderField = "ENVIRONMENT"
)

func (e RedisInstanceOrderField) IsValid() bool {
	switch e {
	case RedisInstanceOrderFieldName, RedisInstanceOrderFieldEnvironment:
		return true
	}
	return false
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
