package opensearch

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
	OpenSearchConnection       = pagination.Connection[*OpenSearch]
	OpenSearchEdge             = pagination.Edge[*OpenSearch]
	OpenSearchAccessConnection = pagination.Connection[*OpenSearchAccess]
	OpenSearchAccessEdge       = pagination.Edge[*OpenSearchAccess]
)

type OpenSearch struct {
	Name                  string              `json:"name"`
	Status                *OpenSearchStatus   `json:"status"`
	TerminationProtection bool                `json:"terminationProtection"`
	TeamSlug              slug.Slug           `json:"-"`
	EnvironmentName       string              `json:"-"`
	WorkloadReference     *workload.Reference `json:"-"`
	AivenProject          string              `json:"-"`
}

func (OpenSearch) IsPersistence() {}
func (OpenSearch) IsSearchNode()  {}
func (OpenSearch) IsNode()        {}

func (r *OpenSearch) GetName() string { return r.Name }

func (r *OpenSearch) GetNamespace() string { return r.TeamSlug.String() }

func (r *OpenSearch) GetLabels() map[string]string { return nil }

func (r *OpenSearch) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (r *OpenSearch) DeepCopyObject() runtime.Object {
	return r
}

func (o OpenSearch) ID() ident.Ident {
	return newIdent(o.TeamSlug, o.EnvironmentName, o.Name)
}

type OpenSearchAccess struct {
	Access            string              `json:"access"`
	TeamSlug          slug.Slug           `json:"-"`
	EnvironmentName   string              `json:"-"`
	WorkloadReference *workload.Reference `json:"-"`
}

type OpenSearchStatus struct {
	State      string             `json:"state"`
	Conditions []metav1.Condition `json:"conditions"`
}

type OpenSearchOrder struct {
	Field     OpenSearchOrderField `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type OpenSearchOrderField string

func (e OpenSearchOrderField) IsValid() bool {
	return SortFilterOpenSearch.SupportsSort(e)
}

func (e OpenSearchOrderField) String() string {
	return string(e)
}

func (e *OpenSearchOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = OpenSearchOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchOrderField", str)
	}
	return nil
}

func (e OpenSearchOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type OpenSearchAccessOrder struct {
	Field     OpenSearchAccessOrderField `json:"field"`
	Direction model.OrderDirection       `json:"direction"`
}

type OpenSearchAccessOrderField string

func (e OpenSearchAccessOrderField) IsValid() bool {
	return SortFilterOpenSearchAccess.SupportsSort(e)
}

func (e OpenSearchAccessOrderField) String() string {
	return string(e)
}

func (e *OpenSearchAccessOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = OpenSearchAccessOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchAccessOrderField", str)
	}
	return nil
}

func (e OpenSearchAccessOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toOpenSearch(u *unstructured.Unstructured, envName string) (*OpenSearch, error) {
	obj := &aiven_io_v1alpha1.OpenSearch{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to OpenSearch: %w", err)
	}

	// Liberator doesn't contain this field, so we read it directly from the unstructured object
	terminationProtection, _, _ := unstructured.NestedBool(u.Object, "spec", "terminationProtection")

	return &OpenSearch{
		Name:                  obj.Name,
		EnvironmentName:       envName,
		TerminationProtection: terminationProtection,
		Status: &OpenSearchStatus{
			Conditions: obj.Status.Conditions,
			State:      obj.Status.State,
		},
		TeamSlug:          slug.Slug(obj.GetNamespace()),
		WorkloadReference: workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
		AivenProject:      obj.Spec.Project,
	}, nil
}

type TeamInventoryCountOpenSearches struct {
	Total int
}
