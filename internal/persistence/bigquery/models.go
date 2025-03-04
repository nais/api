package bigquery

import (
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	bigquery_nais_io_v1 "github.com/nais/liberator/pkg/apis/google.nais.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

type (
	BigQueryDatasetConnection       = pagination.Connection[*BigQueryDataset]
	BigQueryDatasetEdge             = pagination.Edge[*BigQueryDataset]
	BigQueryDatasetAccessConnection = pagination.Connection[*BigQueryDatasetAccess]
	BigQueryDatasetAccessEdge       = pagination.Edge[*BigQueryDatasetAccess]
)

type BigQueryDataset struct {
	// Name equals to the Instance name, not the kubernetes resource name
	Name              string                   `json:"name"`
	Description       *string                  `json:"description,omitempty"`
	CascadingDelete   bool                     `json:"cascadingDelete"`
	Location          string                   `json:"location"`
	Status            *BigQueryDatasetStatus   `json:"status"`
	Access            []*BigQueryDatasetAccess `json:"-"`
	TeamSlug          slug.Slug                `json:"-"`
	EnvironmentName   string                   `json:"-"`
	WorkloadReference *workload.Reference      `json:"-"`
	ProjectID         string                   `json:"-"`
	K8sResourceName   string                   `json:"-"`
}

func (BigQueryDataset) IsPersistence() {}
func (BigQueryDataset) IsSearchNode()  {}
func (BigQueryDataset) IsNode()        {}

func (b *BigQueryDataset) GetName() string { return b.Name }

func (b *BigQueryDataset) GetNamespace() string { return b.TeamSlug.String() }

func (b *BigQueryDataset) GetLabels() map[string]string { return nil }

func (b *BigQueryDataset) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (b *BigQueryDataset) DeepCopyObject() runtime.Object {
	return b
}

func (b BigQueryDataset) ID() ident.Ident {
	return newIdent(b.TeamSlug, b.EnvironmentName, b.K8sResourceName)
}

type BigQueryDatasetAccess struct {
	Role  string `json:"role"`
	Email string `json:"email"`
}

type BigQueryDatasetStatus struct {
	CreationTime     time.Time          `json:"creationTime"`
	LastModifiedTime *time.Time         `json:"lastModifiedTime,omitempty"`
	Conditions       []metav1.Condition `json:"-"`
}

type BigQueryDatasetOrder struct {
	Field     BigQueryDatasetOrderField `json:"field"`
	Direction model.OrderDirection      `json:"direction"`
}

type BigQueryDatasetOrderField string

func (e BigQueryDatasetOrderField) IsValid() bool {
	return SortFilter.Supports(e)
}

func (e BigQueryDatasetOrderField) String() string {
	return string(e)
}

func (e *BigQueryDatasetOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = BigQueryDatasetOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid BigQueryDatasetOrderField", str)
	}
	return nil
}

func (e BigQueryDatasetOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type BigQueryDatasetAccessOrder struct {
	Field     BigQueryDatasetAccessOrderField `json:"field"`
	Direction model.OrderDirection            `json:"direction"`
}

type BigQueryDatasetAccessOrderField string

func (e BigQueryDatasetAccessOrderField) IsValid() bool {
	return SortFilterAccess.Supports(e)
}

func (e BigQueryDatasetAccessOrderField) String() string {
	return string(e)
}

func (e *BigQueryDatasetAccessOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = BigQueryDatasetAccessOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid BigQueryDatasetAccessOrderField", str)
	}
	return nil
}

func (e BigQueryDatasetAccessOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toBigQueryDatasetAccess(access []bigquery_nais_io_v1.DatasetAccess) []*BigQueryDatasetAccess {
	ret := make([]*BigQueryDatasetAccess, len(access))
	for i, a := range access {
		ret[i] = &BigQueryDatasetAccess{
			Role:  a.Role,
			Email: a.UserByEmail,
		}
	}
	return ret
}

func toBigQueryDatasetStatus(s bigquery_nais_io_v1.BigQueryDatasetStatus) *BigQueryDatasetStatus {
	ret := &BigQueryDatasetStatus{
		CreationTime: time.Unix(int64(s.CreationTime), 0),
		Conditions:   s.Conditions,
	}

	if s.LastModifiedTime != 0 {
		ret.LastModifiedTime = ptr.To(time.Unix(int64(s.LastModifiedTime), 0))
	}

	return ret
}

func toBigQueryDataset(u *unstructured.Unstructured, environmentName string) (*BigQueryDataset, error) {
	obj := &bigquery_nais_io_v1.BigQueryDataset{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to BigQueryDataset: %w", err)
	}

	ret := &BigQueryDataset{
		Name:              obj.Spec.Name,
		K8sResourceName:   obj.Name,
		CascadingDelete:   obj.Spec.CascadingDelete,
		Access:            toBigQueryDatasetAccess(obj.Spec.Access),
		Location:          obj.Spec.Location,
		Status:            toBigQueryDatasetStatus(obj.Status),
		TeamSlug:          slug.Slug(obj.GetNamespace()),
		EnvironmentName:   environmentName,
		WorkloadReference: workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
		ProjectID:         obj.Spec.Project,
	}

	if obj.Spec.Description != "" {
		ret.Description = &obj.Spec.Description
	}

	return ret, nil
}

type TeamInventoryCountBigQueryDatasets struct {
	Total int
}
