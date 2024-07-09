package opensearch

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
	OpenSearchConnection       = pagination.Connection[*OpenSearch]
	OpenSearchEdge             = pagination.Edge[*OpenSearch]
	OpenSearchAccessConnection = pagination.Connection[*OpenSearchAccess]
	OpenSearchAccessEdge       = pagination.Edge[*OpenSearchAccess]
)

type OpenSearch struct {
	Name            string                 `json:"name"`
	Workload        workload.Workload      `json:"workload,omitempty"`
	Status          OpenSearchStatus       `json:"status"`
	TeamSlug        slug.Slug              `json:"-"`
	EnvironmentName string                 `json:"-"`
	OwnerReference  *metav1.OwnerReference `json:"-"`
}

func (OpenSearch) IsPersistence() {}

func (OpenSearch) IsNode() {}

func (o OpenSearch) ID() ident.Ident {
	return newIdent(o.TeamSlug, o.EnvironmentName, o.Name)
}

type OpenSearchAccess struct {
	Access          string                 `json:"access"`
	TeamSlug        slug.Slug              `json:"-"`
	EnvironmentName string                 `json:"-"`
	OwnerReference  *metav1.OwnerReference `json:"-"`
}

type OpenSearchStatus struct {
	State      string             `json:"state"`
	Conditions []metav1.Condition `json:"conditions"`
}

type OpenSearchOrder struct {
	Field     OpenSearchOrderField   `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type OpenSearchOrderField string

const (
	OpenSearchOrderFieldName        OpenSearchOrderField = "NAME"
	OpenSearchOrderFieldEnvironment OpenSearchOrderField = "ENVIRONMENT"
)

func (e OpenSearchOrderField) IsValid() bool {
	switch e {
	case OpenSearchOrderFieldName, OpenSearchOrderFieldEnvironment:
		return true
	}
	return false
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
	Direction modelv1.OrderDirection     `json:"direction"`
}

type OpenSearchAccessOrderField string

const (
	OpenSearchAccessOrderFieldAccess   OpenSearchAccessOrderField = "ACCESS"
	OpenSearchAccessOrderFieldWorkload OpenSearchAccessOrderField = "WORKLOAD"
)

func (e OpenSearchAccessOrderField) IsValid() bool {
	switch e {
	case OpenSearchAccessOrderFieldAccess, OpenSearchAccessOrderFieldWorkload:
		return true
	}
	return false
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
	openSearch := &aiven_io_v1alpha1.OpenSearch{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, openSearch); err != nil {
		return nil, fmt.Errorf("converting to OpenSearch: %w", err)
	}

	teamSlug := openSearch.GetNamespace()

	r := &OpenSearch{
		Name:            openSearch.Name,
		EnvironmentName: envName,
		Status: OpenSearchStatus{
			Conditions: openSearch.Status.Conditions,
			State:      openSearch.Status.State,
		},
		TeamSlug:       slug.Slug(teamSlug),
		OwnerReference: persistence.OwnerReference(openSearch.OwnerReferences),
	}

	return r, nil
}
