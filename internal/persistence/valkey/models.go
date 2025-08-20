package valkey

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
	ValkeyConnection       = pagination.Connection[*Valkey]
	ValkeyEdge             = pagination.Edge[*Valkey]
	ValkeyAccessConnection = pagination.Connection[*ValkeyAccess]
	ValkeyAccessEdge       = pagination.Edge[*ValkeyAccess]
)

type Valkey struct {
	Name                  string              `json:"name"`
	Status                *ValkeyStatus       `json:"status"`
	TerminationProtection bool                `json:"terminationProtection"`
	Tier                  ValkeyTier          `json:"tier"`
	Size                  ValkeySize          `json:"size"`
	TeamSlug              slug.Slug           `json:"-"`
	EnvironmentName       string              `json:"-"`
	WorkloadReference     *workload.Reference `json:"-"`
	AivenProject          string              `json:"-"`
}

func (Valkey) IsPersistence() {}
func (Valkey) IsSearchNode()  {}
func (Valkey) IsNode()        {}

func (r *Valkey) GetName() string { return r.Name }

func (r *Valkey) GetNamespace() string { return r.TeamSlug.String() }

func (r *Valkey) GetLabels() map[string]string { return nil }

func (r *Valkey) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (r *Valkey) DeepCopyObject() runtime.Object {
	return r
}

func (r Valkey) ID() ident.Ident {
	return newIdent(r.TeamSlug, r.EnvironmentName, r.Name)
}

type ValkeyAccess struct {
	Access            string              `json:"access"`
	TeamSlug          slug.Slug           `json:"-"`
	EnvironmentName   string              `json:"-"`
	WorkloadReference *workload.Reference `json:"-"`
}

type ValkeyStatus struct {
	State      string             `json:"state"`
	Conditions []metav1.Condition `json:"conditions"`
}

type ValkeyOrder struct {
	Field     ValkeyOrderField     `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type ValkeyOrderField string

func (e ValkeyOrderField) IsValid() bool {
	return SortFilterValkey.SupportsSort(e)
}

func (e ValkeyOrderField) String() string {
	return string(e)
}

func (e *ValkeyOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyOrderField", str)
	}
	return nil
}

func (e ValkeyOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ValkeyAccessOrder struct {
	Field     ValkeyAccessOrderField `json:"field"`
	Direction model.OrderDirection   `json:"direction"`
}

type ValkeyAccessOrderField string

func (e ValkeyAccessOrderField) IsValid() bool {
	return SortFilterValkeyAccess.SupportsSort(e)
}

func (e ValkeyAccessOrderField) String() string {
	return string(e)
}

func (e *ValkeyAccessOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyAccessOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyAccessOrderField", str)
	}
	return nil
}

func (e ValkeyAccessOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toValkey(u *unstructured.Unstructured, envName string) (*Valkey, error) {
	obj := &aiven_io_v1alpha1.Valkey{}

	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, obj); err != nil {
		return nil, fmt.Errorf("converting to Valkey instance: %w", err)
	}

	// Liberator doesn't contain this field, so we read it directly from the unstructured object
	terminationProtection, _, _ := unstructured.NestedBool(u.Object, "spec", "terminationProtection")

	tier, size, err := tierAndSizeFromPlan(obj.Spec.Plan)
	if err != nil {
		return nil, err
	}

	return &Valkey{
		Name:                  obj.Name,
		EnvironmentName:       envName,
		TerminationProtection: terminationProtection,
		Status: &ValkeyStatus{
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

type TeamInventoryCountValkeys struct {
	Total int
}

type ValkeyInput struct {
	Name            string                 `json:"name"`
	EnvironmentName string                 `json:"environmentName"`
	TeamSlug        slug.Slug              `json:"teamSlug"`
	Tier            ValkeyTier             `json:"tier"`
	Size            ValkeySize             `json:"size"`
	MaxMemoryPolicy *ValkeyMaxMemoryPolicy `json:"maxMemoryPolicy,omitempty"`
}

func (v *ValkeyInput) Validate(ctx context.Context) error {
	verr := validate.New()
	v.Name = strings.TrimSpace(v.Name)
	v.EnvironmentName = strings.TrimSpace(v.EnvironmentName)

	if v.Name == "" {
		verr.Add("name", "Name must not be empty.")
	}
	if v.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if v.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}

	if !v.Tier.IsValid() {
		verr.Add("tier", "Invalid Valkey tier: %s.", v.Tier)
	}

	if !v.Size.IsValid() {
		verr.Add("size", "Invalid Valkey size: %s.", v.Size)
	}
	if v.MaxMemoryPolicy != nil && !v.MaxMemoryPolicy.IsValid() {
		verr.Add("version", "Invalid Valkey version: %s.", v.MaxMemoryPolicy.String())
	}

	return verr.NilIfEmpty()
}

type CreateValkeyInput struct {
	ValkeyInput
}

type CreateValkeyPayload struct {
	Valkey *Valkey `json:"valkey"`
}

type ValkeyMaxMemoryPolicy string

const (
	ValkeyMaxMemoryPolicyAllkeysLfu     ValkeyMaxMemoryPolicy = "ALLKEYS_LFU"
	ValkeyMaxMemoryPolicyAllkeysLru     ValkeyMaxMemoryPolicy = "ALLKEYS_LRU"
	ValkeyMaxMemoryPolicyAllkeysRandom  ValkeyMaxMemoryPolicy = "ALLKEYS_RANDOM"
	ValkeyMaxMemoryPolicyNoEviction     ValkeyMaxMemoryPolicy = "NO_EVICTION"
	ValkeyMaxMemoryPolicyVolatileLfu    ValkeyMaxMemoryPolicy = "VOLATILE_LFU"
	ValkeyMaxMemoryPolicyVolatileLru    ValkeyMaxMemoryPolicy = "VOLATILE_LRU"
	ValkeyMaxMemoryPolicyVolatileRandom ValkeyMaxMemoryPolicy = "VOLATILE_RANDOM"
	ValkeyMaxMemoryPolicyVolatileTTL    ValkeyMaxMemoryPolicy = "VOLATILE_TTL"
)

var AllValkeyMaxMemoryPolicy = []ValkeyMaxMemoryPolicy{
	ValkeyMaxMemoryPolicyAllkeysLfu,
	ValkeyMaxMemoryPolicyAllkeysLru,
	ValkeyMaxMemoryPolicyAllkeysRandom,
	ValkeyMaxMemoryPolicyNoEviction,
	ValkeyMaxMemoryPolicyVolatileLfu,
	ValkeyMaxMemoryPolicyVolatileLru,
	ValkeyMaxMemoryPolicyVolatileRandom,
	ValkeyMaxMemoryPolicyVolatileTTL,
}

func (e ValkeyMaxMemoryPolicy) IsValid() bool {
	switch e {
	case ValkeyMaxMemoryPolicyAllkeysLfu,
		ValkeyMaxMemoryPolicyAllkeysLru,
		ValkeyMaxMemoryPolicyAllkeysRandom,
		ValkeyMaxMemoryPolicyNoEviction,
		ValkeyMaxMemoryPolicyVolatileLfu,
		ValkeyMaxMemoryPolicyVolatileLru,
		ValkeyMaxMemoryPolicyVolatileRandom,
		ValkeyMaxMemoryPolicyVolatileTTL:
		return true
	}
	return false
}

func (e ValkeyMaxMemoryPolicy) String() string {
	return string(e)
}

func (e *ValkeyMaxMemoryPolicy) UnmarshalGQL(v any) error {
	str,
		ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyMaxMemoryPolicy(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyMaxMemoryPolicy",
			str)
	}
	return nil
}

func (e ValkeyMaxMemoryPolicy) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ValkeySize string

const (
	ValkeySizeRAM1gb   ValkeySize = "RAM_1GB"
	ValkeySizeRAM4gb   ValkeySize = "RAM_4GB"
	ValkeySizeRAM8gb   ValkeySize = "RAM_8GB"
	ValkeySizeRAM14gb  ValkeySize = "RAM_14GB"
	ValkeySizeRAM28gb  ValkeySize = "RAM_28GB"
	ValkeySizeRAM56gb  ValkeySize = "RAM_56GB"
	ValkeySizeRAM112gb ValkeySize = "RAM_112GB"
	ValkeySizeRAM200gb ValkeySize = "RAM_200GB"
)

var AllValkeySize = []ValkeySize{
	ValkeySizeRAM1gb,
	ValkeySizeRAM4gb,
	ValkeySizeRAM8gb,
	ValkeySizeRAM14gb,
	ValkeySizeRAM28gb,
	ValkeySizeRAM56gb,
	ValkeySizeRAM112gb,
	ValkeySizeRAM200gb,
}

func (e ValkeySize) IsValid() bool {
	switch e {
	case ValkeySizeRAM1gb, ValkeySizeRAM4gb, ValkeySizeRAM8gb, ValkeySizeRAM14gb, ValkeySizeRAM28gb, ValkeySizeRAM56gb, ValkeySizeRAM112gb, ValkeySizeRAM200gb:
		return true
	}
	return false
}

func (e ValkeySize) String() string {
	return string(e)
}

func (e *ValkeySize) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeySize(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeySize", str)
	}
	return nil
}

func (e ValkeySize) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ValkeyTier string

const (
	ValkeyTierSingleNode       ValkeyTier = "SINGLE_NODE"
	ValkeyTierHighAvailability ValkeyTier = "HIGH_AVAILABILITY"
)

var AllValkeyTier = []ValkeyTier{
	ValkeyTierSingleNode,
	ValkeyTierHighAvailability,
}

func (e ValkeyTier) IsValid() bool {
	switch e {
	case ValkeyTierSingleNode, ValkeyTierHighAvailability:
		return true
	}
	return false
}

func (e ValkeyTier) String() string {
	return string(e)
}

func (e *ValkeyTier) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyTier(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyTier", str)
	}
	return nil
}

func (e ValkeyTier) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type UpdateValkeyInput struct {
	ValkeyInput
}

type UpdateValkeyPayload struct {
	Valkey *Valkey `json:"valkey"`
}
