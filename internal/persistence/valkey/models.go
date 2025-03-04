package valkey

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
	ValkeyInstanceConnection       = pagination.Connection[*ValkeyInstance]
	ValkeyInstanceEdge             = pagination.Edge[*ValkeyInstance]
	ValkeyInstanceAccessConnection = pagination.Connection[*ValkeyInstanceAccess]
	ValkeyInstanceAccessEdge       = pagination.Edge[*ValkeyInstanceAccess]
)

type ValkeyInstance struct {
	Name              string                `json:"name"`
	Status            *ValkeyInstanceStatus `json:"status"`
	TeamSlug          slug.Slug             `json:"-"`
	EnvironmentName   string                `json:"-"`
	WorkloadReference *workload.Reference   `json:"-"`
}

func (ValkeyInstance) IsPersistence() {}
func (ValkeyInstance) IsSearchNode()  {}
func (ValkeyInstance) IsNode()        {}

func (r *ValkeyInstance) GetName() string { return r.Name }

func (r *ValkeyInstance) GetNamespace() string { return r.TeamSlug.String() }

func (r *ValkeyInstance) GetLabels() map[string]string { return nil }

func (r *ValkeyInstance) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (r *ValkeyInstance) DeepCopyObject() runtime.Object {
	return r
}

func (r ValkeyInstance) ID() ident.Ident {
	return newIdent(r.TeamSlug, r.EnvironmentName, r.Name)
}

type ValkeyInstanceAccess struct {
	Access            string              `json:"access"`
	TeamSlug          slug.Slug           `json:"-"`
	EnvironmentName   string              `json:"-"`
	WorkloadReference *workload.Reference `json:"-"`
}

type ValkeyInstanceStatus struct {
	State      string             `json:"state"`
	Conditions []metav1.Condition `json:"conditions"`
}

type ValkeyInstanceOrder struct {
	Field     ValkeyInstanceOrderField `json:"field"`
	Direction model.OrderDirection     `json:"direction"`
}

type ValkeyInstanceOrderField string

func (e ValkeyInstanceOrderField) IsValid() bool {
	return SortFilterValkeyInstance.SupportsSort(e)
}

func (e ValkeyInstanceOrderField) String() string {
	return string(e)
}

func (e *ValkeyInstanceOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyInstanceOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyInstanceOrderField", str)
	}
	return nil
}

func (e ValkeyInstanceOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ValkeyInstanceAccessOrder struct {
	Field     ValkeyInstanceAccessOrderField `json:"field"`
	Direction model.OrderDirection           `json:"direction"`
}

type ValkeyInstanceAccessOrderField string

func (e ValkeyInstanceAccessOrderField) IsValid() bool {
	return SortFilterValkeyInstanceAccess.SupportsSort(e)
}

func (e ValkeyInstanceAccessOrderField) String() string {
	return string(e)
}

func (e *ValkeyInstanceAccessOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyInstanceAccessOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyInstanceAccessOrderField", str)
	}
	return nil
}

func (e ValkeyInstanceAccessOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toValkeyInstance(u *unstructured.Unstructured, envName string) (*ValkeyInstance, error) {
	obj := &aiven_io_v1alpha1.Valkey{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to Valkey instance: %w", err)
	}

	return &ValkeyInstance{
		Name:            obj.Name,
		EnvironmentName: envName,
		Status: &ValkeyInstanceStatus{
			Conditions: obj.Status.Conditions,
			State:      obj.Status.State,
		},
		TeamSlug:          slug.Slug(obj.GetNamespace()),
		WorkloadReference: workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
	}, nil
}

type TeamInventoryCountValkeyInstances struct {
	Total int
}
