package opensearch

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
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
	Tier                  OpenSearchTier      `json:"tier"`
	Size                  OpenSearchSize      `json:"size"`
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

	tier, size, err := tierAndSizeFromPlan(obj.Spec.Plan)
	if err != nil {
		return nil, fmt.Errorf("converting to plan: %w", err)
	}

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
		Tier:              tier,
		Size:              size,
	}, nil
}

type TeamInventoryCountOpenSearches struct {
	Total int
}

type OpenSearchMetadataInput struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
}

func (v *OpenSearchMetadataInput) Validate(ctx context.Context) error {
	return v.ValidationErrors(ctx).NilIfEmpty()
}

func (o *OpenSearchMetadataInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
	verr := validate.New()
	o.Name = strings.TrimSpace(o.Name)
	o.EnvironmentName = strings.TrimSpace(o.EnvironmentName)

	if o.Name == "" {
		verr.Add("name", "Name must not be empty.")
	}
	if o.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if o.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}

	return verr
}

type OpenSearchInput struct {
	OpenSearchMetadataInput
	Tier    OpenSearchTier          `json:"tier"`
	Size    OpenSearchSize          `json:"size"`
	Version *OpenSearchMajorVersion `json:"version,omitempty"`
}

func (o *OpenSearchInput) Validate(ctx context.Context) error {
	verr := o.OpenSearchMetadataInput.ValidationErrors(ctx)

	if !o.Tier.IsValid() {
		verr.Add("tier", "Invalid OpenSearch tier: %s.", o.Tier)
	}

	if !o.Size.IsValid() {
		verr.Add("size", "Invalid OpenSearch size: %s.", o.Size)
	}
	if o.Version != nil && !o.Version.IsValid() {
		verr.Add("version", "Invalid OpenSearch version: %s.", o.Version.String())
	}

	return verr.NilIfEmpty()
}

type CreateOpenSearchInput struct {
	OpenSearchInput
}

type CreateOpenSearchPayload struct {
	OpenSearch *OpenSearch `json:"openSearch"`
}

type OpenSearchMajorVersion string

const (
	OpenSearchMajorVersionV2 OpenSearchMajorVersion = "V2"
)

var AllOpenSearchMajorVersion = []OpenSearchMajorVersion{
	OpenSearchMajorVersionV2,
}

func (e OpenSearchMajorVersion) IsValid() bool {
	switch e {
	case OpenSearchMajorVersionV2:
		return true
	}
	return false
}

func (e OpenSearchMajorVersion) String() string {
	return string(e)
}

func (e *OpenSearchMajorVersion) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = OpenSearchMajorVersion(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchMajorVersion", str)
	}
	return nil
}

func (e OpenSearchMajorVersion) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type OpenSearchSize string

const (
	OpenSearchSizeRAM4gb  OpenSearchSize = "RAM_4GB"
	OpenSearchSizeRAM8gb  OpenSearchSize = "RAM_8GB"
	OpenSearchSizeRAM16gb OpenSearchSize = "RAM_16GB"
	OpenSearchSizeRAM32gb OpenSearchSize = "RAM_32GB"
	OpenSearchSizeRAM64gb OpenSearchSize = "RAM_64GB"
)

var AllOpenSearchSize = []OpenSearchSize{
	OpenSearchSizeRAM4gb,
	OpenSearchSizeRAM8gb,
	OpenSearchSizeRAM16gb,
	OpenSearchSizeRAM32gb,
	OpenSearchSizeRAM64gb,
}

func (e OpenSearchSize) IsValid() bool {
	switch e {
	case OpenSearchSizeRAM4gb, OpenSearchSizeRAM8gb, OpenSearchSizeRAM16gb, OpenSearchSizeRAM32gb, OpenSearchSizeRAM64gb:
		return true
	}
	return false
}

func (e OpenSearchSize) String() string {
	return string(e)
}

func (e *OpenSearchSize) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = OpenSearchSize(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchSize", str)
	}
	return nil
}

func (e OpenSearchSize) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type OpenSearchTier string

const (
	OpenSearchTierSingleNode       OpenSearchTier = "SINGLE_NODE"
	OpenSearchTierHighAvailability OpenSearchTier = "HIGH_AVAILABILITY"
)

var AllOpenSearchTier = []OpenSearchTier{
	OpenSearchTierSingleNode,
	OpenSearchTierHighAvailability,
}

func (e OpenSearchTier) IsValid() bool {
	switch e {
	case OpenSearchTierSingleNode, OpenSearchTierHighAvailability:
		return true
	}
	return false
}

func (e OpenSearchTier) String() string {
	return string(e)
}

func (e *OpenSearchTier) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = OpenSearchTier(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchTier", str)
	}
	return nil
}

func (e OpenSearchTier) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type UpdateOpenSearchInput struct{ OpenSearchInput }

type UpdateOpenSearchPayload struct {
	OpenSearch *OpenSearch `json:"openSearch"`
}

type DeleteOpenSearchInput struct {
	OpenSearchMetadataInput
}

type DeleteOpenSearchPayload struct {
	OpenSearchDeleted *bool `json:"openSearchDeleted,omitempty"`
}
