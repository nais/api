package workload

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	WorkloadConnection = pagination.Connection[Workload]
	WorkloadEdge       = pagination.Edge[Workload]
)

type Workload interface {
	model.Node
	activitylog.ActivityLogger
	IsWorkload()
	GetName() string
	GetEnvironmentName() string
	GetTeamSlug() slug.Slug
	GetImageString() string
	GetAccessPolicy() *nais_io_v1.AccessPolicy
	GetConditions() []metav1.Condition
	GetAnnotations() map[string]string
	GetRolloutCompleteTime() int64
	GetType() Type
	GetLogging() *nais_io_v1.Logging

	// GetSecrets returns a list of secret names used by the workload
	GetSecrets() []string

	// GetConfigs returns a list of config names used by the workload
	GetConfigs() []string
	Image() *ContainerImage
}

type Base struct {
	Name                string                   `json:"name"`
	EnvironmentName     string                   `json:"-"`
	TeamSlug            slug.Slug                `json:"-"`
	ImageString         string                   `json:"-"`
	ImageDigest         string                   `json:"-"`
	Conditions          []metav1.Condition       `json:"-"`
	AccessPolicy        *nais_io_v1.AccessPolicy `json:"-"`
	Annotations         map[string]string        `json:"-"`
	RolloutCompleteTime int64                    `json:"-"`
	Type                Type                     `json:"-"`
	Logging             *nais_io_v1.Logging      `json:"-"`
	DeletionStartedAt   *time.Time               `json:"deletedAt"`
}

func (b Base) Image() *ContainerImage {
	return NewContainerImageWithDigest(b.ImageString, b.ImageDigest)
}

func (b Base) GetName() string                           { return b.Name }
func (b Base) GetEnvironmentName() string                { return b.EnvironmentName }
func (b Base) GetConditions() []metav1.Condition         { return b.Conditions }
func (b Base) GetTeamSlug() slug.Slug                    { return b.TeamSlug }
func (b Base) GetImageString() string                    { return b.ImageString }
func (b Base) GetAccessPolicy() *nais_io_v1.AccessPolicy { return b.AccessPolicy }
func (b Base) GetAnnotations() map[string]string         { return b.Annotations }
func (b Base) GetRolloutCompleteTime() int64             { return b.RolloutCompleteTime }
func (b Base) GetType() Type                             { return b.Type }
func (b Base) GetLogging() *nais_io_v1.Logging           { return b.Logging }

type ContainerImage struct {
	Name   string  `json:"name"`
	Tag    string  `json:"tag"`
	Digest *string `json:"digest,omitempty"`
	HasTag bool    `json:"-"`
}

func (ContainerImage) IsNode() {}
func (c ContainerImage) Ref() string {
	ref := c.Name
	if c.HasTag {
		ref += ":" + c.Tag
	}
	if c.Digest != nil && *c.Digest != "" {
		ref += "@" + *c.Digest
	}
	return ref
}

func (c ContainerImage) ID() ident.Ident {
	return newImageIdent(c.stableRef())
}

func (ContainerImage) IsActivityLogger() {}

func (c ContainerImage) stableRef() string {
	if c.HasTag {
		return c.Name + ":" + c.Tag
	}
	return c.Name + ":"
}

func NewContainerImage(image string) *ContainerImage {
	name, tag, digest, hasTag := SplitImage(image)
	ret := &ContainerImage{
		Name:   name,
		Tag:    tag,
		HasTag: hasTag,
	}
	if digest != "" {
		ret.Digest = &digest
	}
	return ret
}

func NewContainerImageWithDigest(image, imageID string) *ContainerImage {
	ret := NewContainerImage(image)
	if digest := DigestFromImageID(imageID); digest != "" {
		ret.Digest = &digest
	}
	return ret
}

func SplitImage(image string) (name, tag, digest string, hasTag bool) {
	before, after, ok := strings.Cut(image, "@")
	if ok {
		image = before
		digest = after
	}

	name = image
	tag = "latest"

	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		name = image[:lastColon]
		tagPart := image[lastColon+1:]
		if tagPart != "" {
			tag = tagPart
			hasTag = true
		}
	}
	return name, tag, digest, hasTag
}

func DigestFromImageID(imageID string) string {
	_, digest, ok := strings.Cut(imageID, "@")
	if ok {
		return digest
	}

	if _, after, ok := strings.Cut(imageID, "://"); ok {
		imageID = after
	}

	if strings.HasPrefix(imageID, "sha256:") {
		return imageID
	}

	return ""
}

// DigestFromPodStatusByAppName returns the digest for the container named appName,
// falling back to any available digest if the named container is not found or has none.
// This prevents sidecars from shadowing the main application container digest.
func DigestFromPodStatusByAppName(appName string, statuses []corev1.ContainerStatus) string {
	for _, cs := range statuses {
		if cs.Name == appName {
			if digest := DigestFromImageID(cs.ImageID); digest != "" {
				return digest
			}
			break
		}
	}
	for _, cs := range statuses {
		if digest := DigestFromImageID(cs.ImageID); digest != "" {
			return digest
		}
	}
	return ""
}

func DigestFromPodStatus(containers []corev1.Container, statuses []corev1.ContainerStatus) string {
	if len(statuses) == 0 {
		return ""
	}

	expectedContainer := ""
	if len(containers) > 0 {
		expectedContainer = containers[0].Name
	}

	if expectedContainer != "" {
		for _, cs := range statuses {
			if cs.Name != expectedContainer {
				continue
			}
			if digest := DigestFromImageID(cs.ImageID); digest != "" {
				return digest
			}
		}
	}

	for _, cs := range statuses {
		if digest := DigestFromImageID(cs.ImageID); digest != "" {
			return digest
		}
	}

	return ""
}

type WorkloadResources interface {
	IsWorkloadResources()
}

type WorkloadResourceQuantity struct {
	CPU    *float64 `json:"cpu"`
	Memory *int64   `json:"memory"`
}

type AuthIntegration interface {
	IsAuthIntegration()
}

type ApplicationAuthIntegrations interface {
	AuthIntegration
}

type JobAuthIntegrations interface {
	AuthIntegration
}

type EntraIDAuthIntegration struct{}

func (EntraIDAuthIntegration) IsAuthIntegration() {}
func (EntraIDAuthIntegration) Name() string {
	return "Microsoft Entra ID"
}

type IDPortenAuthIntegration struct{}

func (IDPortenAuthIntegration) IsAuthIntegration() {}
func (IDPortenAuthIntegration) Name() string {
	return "ID-porten"
}

type MaskinportenAuthIntegration struct{}

func (MaskinportenAuthIntegration) IsAuthIntegration() {}
func (MaskinportenAuthIntegration) Name() string {
	return "Maskinporten"
}

type TokenXAuthIntegration struct{}

func (TokenXAuthIntegration) IsAuthIntegration() {}
func (TokenXAuthIntegration) Name() string {
	return "TokenX"
}

type WorkloadOrder struct {
	Field     WorkloadOrderField   `json:"field"`
	Direction model.OrderDirection `json:"direction"`
}

type WorkloadOrderField string

func (e WorkloadOrderField) IsValid() bool {
	return SortFilter.SupportsSort(e)
}

func (e WorkloadOrderField) String() string {
	return string(e)
}

func (e *WorkloadOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = WorkloadOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid WorkloadOrderField", str)
	}
	return nil
}

func (e WorkloadOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type WorkloadManifest interface {
	IsWorkloadManifest()
}

type Type int

const (
	TypeApplication Type = iota
	TypeJob
)

func (t Type) String() string {
	switch t {
	case TypeApplication:
		return "Application"
	case TypeJob:
		return "Naisjob"
	default:
		return "Unknown"
	}
}

// TypeFromString returns the Type for the given string. If the string does not match any known type, -1 is returned.
func TypeFromString(s string) (Type, error) {
	switch s {
	case "Application":
		return TypeApplication, nil
	case "Naisjob":
		return TypeJob, nil
	default:
		return -1, fmt.Errorf("unknown workload type: %s", s)
	}
}

type EnvironmentWorkloadOrder struct {
	Field     EnvironmentWorkloadOrderField `json:"field"`
	Direction model.OrderDirection          `json:"direction"`
}

type EnvironmentWorkloadOrderField string

func (e EnvironmentWorkloadOrderField) IsValid() bool {
	return SortFilterEnvironment.SupportsSort(e)
}

func (e EnvironmentWorkloadOrderField) String() string {
	return string(e)
}

func (e *EnvironmentWorkloadOrderField) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = EnvironmentWorkloadOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid EnvironmentWorkloadOrderField", str)
	}
	return nil
}

func (e EnvironmentWorkloadOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type Reference struct {
	// Name is the name of the referenced workload.
	Name string

	// Type is the type of the referenced workload.
	Type Type
}

// ReferenceFromOwnerReferences returns a Reference for the first valid owner reference. If none can be found, nil is
// returned.
func ReferenceFromOwnerReferences(ownerReferences []metav1.OwnerReference) *Reference {
	if len(ownerReferences) == 0 {
		return nil
	}

	for _, o := range ownerReferences {
		switch o.Kind {
		case "Naisjob":
			return &Reference{
				Name: o.Name,
				Type: TypeJob,
			}
		case "Application":
			return &Reference{
				Name: o.Name,
				Type: TypeApplication,
			}
		}
	}
	return nil
}

type TeamWorkloadsFilter struct {
	Environments             []string `json:"environments,omitempty"`
	States                   []string `json:"states,omitempty"`
	WorkloadStatusErrorTypes []string `json:"workloadStatusErrorTypes,omitempty"`
}

type UpdateWorkloadEnvironmentVariableInput struct {
	Name  string  `json:"name"`
	Value *string `json:"value,omitempty"`
}

func MergeEnvVars(existing nais_io_v1.EnvVars, updates []*UpdateWorkloadEnvironmentVariableInput) nais_io_v1.EnvVars {
	envMap := make(map[string]nais_io_v1.EnvVar, len(existing))
	for _, e := range existing {
		envMap[e.Name] = e
	}

	for _, u := range updates {
		if u.Value == nil {
			delete(envMap, u.Name)
		} else {
			envMap[u.Name] = nais_io_v1.EnvVar{Name: u.Name, Value: *u.Value}
		}
	}

	result := make(nais_io_v1.EnvVars, 0, len(envMap))
	for _, v := range envMap {
		result = append(result, v)
	}

	slices.SortFunc(result, func(a, b nais_io_v1.EnvVar) int {
		return strings.Compare(a.Name, b.Name)
	})
	return result
}

func EnvVarsEqual(a, b nais_io_v1.EnvVars) bool {
	if len(a) != len(b) {
		return false
	}
	m := make(map[string]string, len(a))
	for _, v := range a {
		m[v.Name] = v.Value
	}
	for _, v := range b {
		if existing, ok := m[v.Name]; !ok || existing != v.Value {
			return false
		}
	}
	return true
}

func EnvVarChangedFields(existing nais_io_v1.EnvVars, merged nais_io_v1.EnvVars) []*activitylog.ResourceChangedField {
	oldMap := make(map[string]string, len(existing))
	for _, v := range existing {
		oldMap[v.Name] = v.Value
	}
	newMap := make(map[string]string, len(merged))
	for _, v := range merged {
		newMap[v.Name] = v.Value
	}

	var fields []*activitylog.ResourceChangedField

	for name, oldVal := range oldMap {
		if newVal, ok := newMap[name]; !ok {
			old := oldVal
			fields = append(fields, &activitylog.ResourceChangedField{Field: "spec.env." + name, OldValue: &old})
		} else if newVal != oldVal {
			old := oldVal
			n := newVal
			fields = append(fields, &activitylog.ResourceChangedField{Field: "spec.env." + name, OldValue: &old, NewValue: &n})
		}
	}

	for name, newVal := range newMap {
		if _, ok := oldMap[name]; !ok {
			n := newVal
			fields = append(fields, &activitylog.ResourceChangedField{Field: "spec.env." + name, NewValue: &n})
		}
	}

	slices.SortFunc(fields, func(a, b *activitylog.ResourceChangedField) int {
		return strings.Compare(a.Field, b.Field)
	})

	return fields
}
