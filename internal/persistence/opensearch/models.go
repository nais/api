package opensearch

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes"
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
	Name                  string                 `json:"name"`
	Status                *OpenSearchStatus      `json:"status"`
	TerminationProtection bool                   `json:"terminationProtection"`
	Tier                  OpenSearchTier         `json:"tier"`
	Memory                OpenSearchMemory       `json:"memory"`
	StorageGB             StorageGB              `json:"storageGB"`
	TeamSlug              slug.Slug              `json:"-"`
	EnvironmentName       string                 `json:"-"`
	WorkloadReference     *workload.Reference    `json:"-"`
	AivenProject          string                 `json:"-"`
	MajorVersion          OpenSearchMajorVersion `json:"-"`
}

func (OpenSearch) IsPersistence()    {}
func (OpenSearch) IsSearchNode()     {}
func (OpenSearch) IsNode()           {}
func (OpenSearch) IsActivityLogger() {}

func (r *OpenSearch) GetName() string { return r.Name }

func (r *OpenSearch) GetNamespace() string { return r.TeamSlug.String() }

func (r *OpenSearch) GetLabels() map[string]string { return nil }

func (r *OpenSearch) FullyQualifiedName() string {
	if strings.HasPrefix(r.Name, namePrefix(r.TeamSlug)) {
		return r.Name
	}
	return instanceNamer(r.TeamSlug, r.Name)
}

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

	machine, err := machineTypeFromPlan(obj.Spec.Plan)
	if err != nil {
		return nil, err
	}

	name := obj.Name
	if kubernetes.HasManagedByConsoleLabel(obj) {
		name = strings.TrimPrefix(obj.GetName(), namePrefix(slug.Slug(obj.GetNamespace())))
	}

	majorVersion := OpenSearchMajorVersion("")
	if v, found, _ := unstructured.NestedString(u.Object, "spec", "userConfig", "opensearch_version"); found {
		version, err := OpenSearchMajorVersionFromAivenString(v)
		if err == nil {
			majorVersion = version
		}
	}

	// default to minimum storage capacity for the selected plan, in case the field is not set explicitly
	storageGB := machine.StorageMin
	if v, found, _ := unstructured.NestedString(u.Object, "spec", "disk_space"); found {
		storageGB, err = StorageGBFromAivenString(v)
		if err != nil {
			return nil, err
		}
	}

	return &OpenSearch{
		Name:                  name,
		EnvironmentName:       envName,
		TerminationProtection: terminationProtection,
		Status: &OpenSearchStatus{
			Conditions: obj.Status.Conditions,
			State:      obj.Status.State,
		},
		TeamSlug:          slug.Slug(obj.GetNamespace()),
		WorkloadReference: workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
		AivenProject:      obj.Spec.Project,
		Tier:              machine.Tier,
		Memory:            machine.Memory,
		MajorVersion:      majorVersion,
		StorageGB:         storageGB,
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
	Tier      OpenSearchTier         `json:"tier"`
	Memory    OpenSearchMemory       `json:"memory"`
	Version   OpenSearchMajorVersion `json:"version"`
	StorageGB StorageGB              `json:"storageGB"`
}

func (o *OpenSearchInput) Validate(ctx context.Context) error {
	verr := o.OpenSearchMetadataInput.ValidationErrors(ctx)

	if !o.Tier.IsValid() {
		verr.Add("tier", "Invalid OpenSearch tier: %s.", o.Tier)
	}
	if !o.Memory.IsValid() {
		verr.Add("memory", "Invalid OpenSearch memory: %s.", o.Memory)
	}
	if !o.Version.IsValid() {
		verr.Add("version", "Invalid OpenSearch version: %s.", o.Version.String())
	}

	machine, err := machineTypeFromTierAndMemory(o.Tier, o.Memory)
	if err != nil {
		verr.Add("memory", "%s", err)
		return verr.NilIfEmpty()
	}

	// hobbyist plan has a fixed storage capacity, so we override any provided value
	if machine.AivenPlan == "hobbyist" {
		o.StorageGB = machine.StorageMin
	}

	if o.StorageGB < machine.StorageMin || o.StorageGB > machine.StorageMax {
		verr.Add("storageGB", "Storage capacity must be between %dG and %dG for tier %q and memory %q.", machine.StorageMin, machine.StorageMax, o.Tier, o.Memory)
	}

	return verr.NilIfEmpty()
}

type StorageGB int

func (o StorageGB) ToAivenString() string {
	return strconv.Itoa(int(o)) + "G"
}

func (o StorageGB) String() string {
	return strconv.Itoa(int(o))
}

func (o StorageGB) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, o)
}

func (o *StorageGB) UnmarshalGQL(v any) error {
	i, err := graphql.UnmarshalInt(v)
	if err != nil {
		return fmt.Errorf("storage capacity must be an integer")
	}
	if i <= 0 {
		return fmt.Errorf("storage capacity must be a positive integer")
	}
	*o = StorageGB(i)
	return nil
}

func StorageGBFromAivenString(s string) (StorageGB, error) {
	i, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(s, "iB"), "G"))
	if err != nil {
		return 0, fmt.Errorf("parsing OpenSearch storage capacity from Aiven string %q: %w", s, err)
	}
	return StorageGB(i), nil
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
	OpenSearchMajorVersionV1 OpenSearchMajorVersion = "V1"
)

func (e OpenSearchMajorVersion) IsDowngradeTo(other OpenSearchMajorVersion) bool {
	if !e.IsValid() || !other.IsValid() {
		return false
	}

	// Since we've already checked if both versions are valid, the Atoi calls should never fail
	our, _ := strconv.Atoi(e.ToAivenString())
	their, _ := strconv.Atoi(other.ToAivenString())

	return our < their
}

func (e OpenSearchMajorVersion) IsValid() bool {
	switch e {
	case OpenSearchMajorVersionV2, OpenSearchMajorVersionV1:
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

// ToAivenString returns the version string without the "V" prefix, e.g. "2" or "1".
func (e OpenSearchMajorVersion) ToAivenString() string {
	return strings.TrimLeft(string(e), "V")
}

func OpenSearchMajorVersionFromAivenString(s string) (OpenSearchMajorVersion, error) {
	parts := strings.Split(s, ".")
	if len(parts) == 0 {
		return "", fmt.Errorf("unexpected Aiven OpenSearch version: %q", s)
	}

	v := OpenSearchMajorVersion("V" + parts[0])
	if !v.IsValid() {
		return "", fmt.Errorf("unsupported Aiven OpenSearch version: %q", s)
	}

	return v, nil
}

type OpenSearchMemory string

const (
	OpenSearchMemoryGB2  OpenSearchMemory = "GB_2"
	OpenSearchMemoryGB4  OpenSearchMemory = "GB_4"
	OpenSearchMemoryGB8  OpenSearchMemory = "GB_8"
	OpenSearchMemoryGB16 OpenSearchMemory = "GB_16"
	OpenSearchMemoryGB32 OpenSearchMemory = "GB_32"
	OpenSearchMemoryGB64 OpenSearchMemory = "GB_64"
)

func (e OpenSearchMemory) IsValid() bool {
	switch e {
	case OpenSearchMemoryGB2, OpenSearchMemoryGB4, OpenSearchMemoryGB8, OpenSearchMemoryGB16, OpenSearchMemoryGB32, OpenSearchMemoryGB64:
		return true
	}
	return false
}

func (e OpenSearchMemory) String() string {
	return string(e)
}

func (e *OpenSearchMemory) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = OpenSearchMemory(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchMemory", str)
	}
	return nil
}

func (e OpenSearchMemory) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type OpenSearchTier string

const (
	OpenSearchTierSingleNode       OpenSearchTier = "SINGLE_NODE"
	OpenSearchTierHighAvailability OpenSearchTier = "HIGH_AVAILABILITY"
)

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

type OpenSearchVersion struct {
	Actual       *string                `json:"actual,omitempty"`
	DesiredMajor OpenSearchMajorVersion `json:"desiredMajor"`
}

type OpenSearchState int

const (
	OpenSearchStateUnknown OpenSearchState = iota
	OpenSearchStateRunning
	OpenSearchStateRebalancing
	OpenSearchStateRebuilding
	OpenSearchStatePoweroff
)

var AllOpenSearchState = []OpenSearchState{
	OpenSearchStatePoweroff,
	OpenSearchStateRebalancing,
	OpenSearchStateRebuilding,
	OpenSearchStateRunning,
	OpenSearchStateUnknown,
}

func (e OpenSearchState) String() string {
	switch e {
	case OpenSearchStatePoweroff:
		return "POWEROFF"
	case OpenSearchStateRebalancing:
		return "REBALANCING"
	case OpenSearchStateRebuilding:
		return "REBUILDING"
	case OpenSearchStateRunning:
		return "RUNNING"
	default:
		return "UNKNOWN"
	}
}

func (e OpenSearchState) IsValid() bool {
	switch e {
	case OpenSearchStatePoweroff, OpenSearchStateRebalancing, OpenSearchStateRebuilding, OpenSearchStateRunning, OpenSearchStateUnknown:
		return true
	}
	return false
}

func (e *OpenSearchState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	switch str {
	case "POWEROFF":
		*e = OpenSearchStatePoweroff
	case "REBALANCING":
		*e = OpenSearchStateRebalancing
	case "REBUILDING":
		*e = OpenSearchStateRebuilding
	case "RUNNING":
		*e = OpenSearchStateRunning
	case "UNKNOWN":
		*e = OpenSearchStateUnknown
	default:
		return fmt.Errorf("%s is not a valid OpenSearchState", str)
	}

	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid OpenSearchState", str)
	}
	return nil
}

func (e OpenSearchState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
