package workload

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	WorkloadConnection = pagination.Connection[Workload]
	WorkloadEdge       = pagination.Edge[Workload]
)

type Workload interface {
	model.Node
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
	Image() *ContainerImage
}

type Base struct {
	Name                string                   `json:"name"`
	EnvironmentName     string                   `json:"-"`
	TeamSlug            slug.Slug                `json:"-"`
	ImageString         string                   `json:"-"`
	Conditions          []metav1.Condition       `json:"-"`
	AccessPolicy        *nais_io_v1.AccessPolicy `json:"-"`
	Annotations         map[string]string        `json:"-"`
	RolloutCompleteTime int64                    `json:"-"`
	Type                Type                     `json:"-"`
	Logging             *nais_io_v1.Logging      `json:"-"`
}

func (b Base) Image() *ContainerImage {
	name, tag, _ := strings.Cut(b.ImageString, ":")
	return &ContainerImage{
		Name: name,
		Tag:  tag,
	}
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
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

func (ContainerImage) IsNode() {}
func (c ContainerImage) Ref() string {
	return c.Name + ":" + c.Tag
}

func (c ContainerImage) ID() ident.Ident {
	return newImageIdent(c.Ref())
}

type WorkloadResources interface {
	IsWorkloadResources()
}

type WorkloadResourceQuantity struct {
	CPU    float64 `json:"cpu"`
	Memory int64   `json:"memory"`
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

const (
	WorkloadOrderFieldName           WorkloadOrderField = "NAME"
	WorkloadOrderFieldStatus         WorkloadOrderField = "STATUS"
	WorkloadOrderFieldEnvironment    WorkloadOrderField = "ENVIRONMENT"
	WorkloadOrderFieldDeploymentTime WorkloadOrderField = "DEPLOYMENT_TIME"
)

var AllWorkloadOrderField = []WorkloadOrderField{
	WorkloadOrderFieldName,
	WorkloadOrderFieldStatus,
	WorkloadOrderFieldEnvironment,
	WorkloadOrderFieldDeploymentTime,
}

func (e WorkloadOrderField) IsValid() bool {
	for _, f := range AllWorkloadOrderField {
		if e == f {
			return true
		}
	}
	return false
}

func (e WorkloadOrderField) String() string {
	return string(e)
}

func (e *WorkloadOrderField) UnmarshalGQL(v interface{}) error {
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
	Environments []string `json:"environments"`
}
