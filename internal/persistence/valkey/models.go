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
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/validate"
	"github.com/nais/api/internal/workload"
	aiven_io_v1alpha1 "github.com/nais/liberator/pkg/apis/aiven.io/v1alpha1"
	"github.com/nais/pgrator/pkg/api"
	naiscrd "github.com/nais/pgrator/pkg/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

type (
	ValkeyConnection       = pagination.Connection[*Valkey]
	ValkeyEdge             = pagination.Edge[*Valkey]
	ValkeyAccessConnection = pagination.Connection[*ValkeyAccess]
	ValkeyAccessEdge       = pagination.Edge[*ValkeyAccess]
)

type Valkey struct {
	Name                  string                `json:"name"`
	Status                *naiscrd.ValkeyStatus `json:"status"`
	TerminationProtection bool                  `json:"terminationProtection"`
	Tier                  ValkeyTier            `json:"tier"`
	Memory                ValkeyMemory          `json:"memory"`
	MaxMemoryPolicy       ValkeyMaxMemoryPolicy `json:"maxMemoryPolicy,omitempty"`
	NotifyKeyspaceEvents  string                `json:"notifyKeyspaceEvents,omitempty"`
	TeamSlug              slug.Slug             `json:"-"`
	EnvironmentName       string                `json:"-"`
	WorkloadReference     *workload.Reference   `json:"-"`
}

func (Valkey) IsPersistence()    {}
func (Valkey) IsSearchNode()     {}
func (Valkey) IsNode()           {}
func (Valkey) IsActivityLogger() {}

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

func (r Valkey) FullyQualifiedName() string {
	if strings.HasPrefix(r.Name, namePrefix(r.TeamSlug)) {
		return r.Name
	}
	return instanceNamer(r.TeamSlug, r.Name)
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

	if len(obj.GetOwnerReferences()) > 0 {
		for _, ownerRef := range obj.GetOwnerReferences() {
			if ownerRef.Kind == "Valkey" {
				return nil, fmt.Errorf("skipping Valkey %s in namespace %s because it has an owner reference", obj.GetName(), obj.GetNamespace())
			}
		}
	}

	// Liberator doesn't contain this field, so we read it directly from the unstructured object
	terminationProtection, _, _ := unstructured.NestedBool(u.Object, specTerminationProtection...)

	maxMemoryPolicyStr, _, _ := unstructured.NestedString(u.Object, specMaxMemoryPolicy...)
	maxMemoryPolicy, err := ValkeyMaxMemoryPolicyFromAivenString(naiscrd.ValkeyMaxMemoryPolicy(maxMemoryPolicyStr))
	if err != nil {
		maxMemoryPolicy = ""
	}

	notifyKeyspaceEvents, _, _ := unstructured.NestedString(u.Object, specNotifyKeyspaceEvents...)

	machine, err := machineTypeFromPlan(obj.Spec.Plan)
	if err != nil {
		return nil, fmt.Errorf("converting from plan: %w", err)
	}

	name := obj.Name
	if kubernetes.HasManagedByConsoleLabel(obj) {
		name = strings.TrimPrefix(obj.GetName(), namePrefix(slug.Slug(obj.GetNamespace())))
	}

	return &Valkey{
		Name:                  name,
		EnvironmentName:       envName,
		TerminationProtection: terminationProtection,
		Status: &naiscrd.ValkeyStatus{
			BaseStatus: api.BaseStatus{
				Conditions: obj.Status.Conditions,
			},
		},
		TeamSlug:             slug.Slug(obj.GetNamespace()),
		WorkloadReference:    workload.ReferenceFromOwnerReferences(obj.GetOwnerReferences()),
		Tier:                 machine.Tier,
		Memory:               machine.Memory,
		MaxMemoryPolicy:      maxMemoryPolicy,
		NotifyKeyspaceEvents: notifyKeyspaceEvents,
	}, nil
}

func toValkeyFromNais(v *naiscrd.Valkey, envName string) (*Valkey, error) {
	var mmp ValkeyMaxMemoryPolicy
	if v.Spec.MaxMemoryPolicy != "" {
		var err error
		mmp, err = ValkeyMaxMemoryPolicyFromAivenString(v.Spec.MaxMemoryPolicy)
		if err != nil {
			return nil, err
		}
	}
	return &Valkey{
		Name:                 v.Name,
		EnvironmentName:      envName,
		Status:               v.Status,
		TeamSlug:             slug.Slug(v.Namespace),
		WorkloadReference:    workload.ReferenceFromOwnerReferences(v.OwnerReferences),
		Tier:                 fromMapperatorTier(v.Spec.Tier),
		Memory:               fromMapperatorMemory(v.Spec.Memory),
		NotifyKeyspaceEvents: v.Spec.NotifyKeyspaceEvents,
		MaxMemoryPolicy:      mmp,
	}, nil
}

type TeamInventoryCountValkeys struct {
	Total int
}

type ValkeyMetadataInput struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"environmentName"`
	TeamSlug        slug.Slug `json:"teamSlug"`
}

func (v *ValkeyMetadataInput) Validate(ctx context.Context) error {
	return v.ValidationErrors(ctx).NilIfEmpty()
}

func (v *ValkeyMetadataInput) ValidationErrors(ctx context.Context) *validate.ValidationErrors {
	verr := validate.New()
	v.Name = strings.TrimSpace(v.Name)
	v.EnvironmentName = strings.TrimSpace(v.EnvironmentName)

	if v.Name == "" {
		verr.Add("name", "Name must not be empty.")
	}
	if errs := validation.IsDNS1123Subdomain(v.Name); len(errs) > 0 {
		verr.Add("name", "Name must consist of lowercase letters, numbers, and hyphens only. It cannot start or end with a hyphen.")
	}
	if v.EnvironmentName == "" {
		verr.Add("environmentName", "Environment name must not be empty.")
	}
	if v.TeamSlug == "" {
		verr.Add("teamSlug", "Team slug must not be empty.")
	}

	return verr
}

type ValkeyInput struct {
	ValkeyMetadataInput
	Tier                 ValkeyTier             `json:"tier"`
	Memory               ValkeyMemory           `json:"memory"`
	MaxMemoryPolicy      *ValkeyMaxMemoryPolicy `json:"maxMemoryPolicy,omitempty"`
	NotifyKeyspaceEvents *string                `json:"notifyKeyspaceEvents,omitempty"`
}

func (v *ValkeyInput) Validate(ctx context.Context) error {
	verr := v.ValkeyMetadataInput.ValidationErrors(ctx)

	if !v.Tier.IsValid() {
		verr.Add("tier", "Invalid Valkey tier: %s.", v.Tier)
	}

	if !v.Memory.IsValid() {
		verr.Add("memory", "Invalid Valkey memory: %s.", v.Memory)
	}
	if v.MaxMemoryPolicy != nil && !v.MaxMemoryPolicy.IsValid() {
		verr.Add("version", "Invalid Valkey max memory policy: %s.", v.MaxMemoryPolicy.String())
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
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyMaxMemoryPolicy(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyMaxMemoryPolicy", str)
	}
	return nil
}

func (e ValkeyMaxMemoryPolicy) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func ValkeyMaxMemoryPolicyFromAivenString(s naiscrd.ValkeyMaxMemoryPolicy) (ValkeyMaxMemoryPolicy, error) {
	for _, policy := range AllValkeyMaxMemoryPolicy {
		if policy.ToAivenString() == string(s) {
			return policy, nil
		}
	}
	return "", fmt.Errorf("invalid ValkeyMaxMemoryPolicy: %s", s)
}

func (e ValkeyMaxMemoryPolicy) ToAivenString() string {
	switch e {
	case ValkeyMaxMemoryPolicyAllkeysLfu:
		return string(naiscrd.ValkeyMaxMemoryPolicyAllkeysLFU)
	case ValkeyMaxMemoryPolicyAllkeysLru:
		return string(naiscrd.ValkeyMaxMemoryPolicyAllkeysLRU)
	case ValkeyMaxMemoryPolicyAllkeysRandom:
		return string(naiscrd.ValkeyMaxMemoryPolicyAllkeysRandom)
	case ValkeyMaxMemoryPolicyNoEviction:
		return string(naiscrd.ValkeyMaxMemoryPolicyNoEviction)
	case ValkeyMaxMemoryPolicyVolatileLfu:
		return string(naiscrd.ValkeyMaxMemoryPolicyVolatileLFU)
	case ValkeyMaxMemoryPolicyVolatileLru:
		return string(naiscrd.ValkeyMaxMemoryPolicyVolatileLRU)
	case ValkeyMaxMemoryPolicyVolatileRandom:
		return string(naiscrd.ValkeyMaxMemoryPolicyVolatileRandom)
	case ValkeyMaxMemoryPolicyVolatileTTL:
		return string(naiscrd.ValkeyMaxMemoryPolicyVolatileTTL)
	default:
		return ""
	}
}

type ValkeyMemory string

const (
	ValkeyMemoryGB1   ValkeyMemory = "GB_1"
	ValkeyMemoryGB4   ValkeyMemory = "GB_4"
	ValkeyMemoryGB8   ValkeyMemory = "GB_8"
	ValkeyMemoryGB14  ValkeyMemory = "GB_14"
	ValkeyMemoryGB28  ValkeyMemory = "GB_28"
	ValkeyMemoryGB56  ValkeyMemory = "GB_56"
	ValkeyMemoryGB112 ValkeyMemory = "GB_112"
	ValkeyMemoryGB200 ValkeyMemory = "GB_200"
)

func (e ValkeyMemory) IsValid() bool {
	switch e {
	case ValkeyMemoryGB1, ValkeyMemoryGB4, ValkeyMemoryGB8, ValkeyMemoryGB14, ValkeyMemoryGB28, ValkeyMemoryGB56, ValkeyMemoryGB112, ValkeyMemoryGB200:
		return true
	}
	return false
}

func (e ValkeyMemory) String() string {
	return string(e)
}

func (e *ValkeyMemory) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ValkeyMemory(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyMemory", str)
	}
	return nil
}

func (e ValkeyMemory) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ValkeyTier string

const (
	ValkeyTierSingleNode       ValkeyTier = "SINGLE_NODE"
	ValkeyTierHighAvailability ValkeyTier = "HIGH_AVAILABILITY"
)

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

type DeleteValkeyInput struct {
	ValkeyMetadataInput
}

type DeleteValkeyPayload struct {
	ValkeyDeleted *bool `json:"valkeyDeleted,omitempty"`
}

type ValkeyState int

const (
	ValkeyStateUnknown ValkeyState = iota
	ValkeyStateRunning
	ValkeyStateRebalancing
	ValkeyStateRebuilding
	ValkeyStatePoweroff
)

var AllValkeyState = []ValkeyState{
	ValkeyStateUnknown,
	ValkeyStateRunning,
	ValkeyStateRebalancing,
	ValkeyStateRebuilding,
	ValkeyStatePoweroff,
}

func (e ValkeyState) IsValid() bool {
	switch e {
	case ValkeyStatePoweroff, ValkeyStateRebalancing, ValkeyStateRebuilding, ValkeyStateRunning, ValkeyStateUnknown:
		return true
	}
	return false
}

func (e ValkeyState) String() string {
	switch e {
	case ValkeyStatePoweroff:
		return "POWEROFF"
	case ValkeyStateRebalancing:
		return "REBALANCING"
	case ValkeyStateRebuilding:
		return "REBUILDING"
	case ValkeyStateRunning:
		return "RUNNING"
	default:
		return "UNKNOWN"
	}
}

func (e *ValkeyState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	switch str {
	case "POWEROFF":
		*e = ValkeyStatePoweroff
	case "REBALANCING":
		*e = ValkeyStateRebalancing
	case "REBUILDING":
		*e = ValkeyStateRebuilding
	case "RUNNING":
		*e = ValkeyStateRunning
	case "UNKNOWN":
		*e = ValkeyStateUnknown
	default:
		return fmt.Errorf("%s is not a valid ValkeyState", str)
	}

	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ValkeyState", str)
	}
	return nil
}

func (e ValkeyState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
