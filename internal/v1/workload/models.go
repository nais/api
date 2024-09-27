package workload

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
)

type (
	WorkloadConnection = pagination.Connection[Workload]
	WorkloadEdge       = pagination.Edge[Workload]
)

type Workload interface {
	modelv1.Node
	IsWorkload()
	GetName() string
	GetEnvironmentName() string
}

type Base struct {
	Name            string    `json:"name"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	ImageString     string    `json:"-"`
}

func (b Base) Image() *ContainerImage {
	name, tag, _ := strings.Cut(b.ImageString, ":")
	return &ContainerImage{
		Name: name,
		Tag:  tag,
	}
}

func (b Base) GetName() string            { return b.Name }
func (b Base) GetEnvironmentName() string { return b.EnvironmentName }

type ContainerImage struct {
	Name string `json:"name"`
	Tag  string `json:"tag"`
}

func (ContainerImage) IsNode() {}
func (c ContainerImage) Ref() string {
	return c.Name + ":" + c.Tag
}

func (c ContainerImage) ID() ident.Ident {
	return newImageIdent(c.Name + ":" + c.Tag)
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

type EntraIDAuthIntegration struct {
	Tmp bool `json:"tmp"`
}

func (EntraIDAuthIntegration) IsAuthIntegration() {}

type IDPortenAuthIntegration struct {
	Tmp bool `json:"tmp"`
}

func (IDPortenAuthIntegration) IsAuthIntegration() {}

type MaskinportenAuthIntegration struct {
	Tmp bool `json:"tmp"`
}

func (MaskinportenAuthIntegration) IsAuthIntegration() {}

type TokenXAuthIntegration struct {
	Tmp bool `json:"tmp"`
}

func (TokenXAuthIntegration) IsAuthIntegration() {}

// Ordering options when fetching workloads.
type WorkloadOrder struct {
	// The field to order items by.
	Field WorkloadOrderField `json:"field"`
	// The direction to order items by.
	Direction modelv1.OrderDirection `json:"direction"`
}

type WorkloadOrderField string

const (
	// Order Workloads by name.
	WorkloadOrderFieldName WorkloadOrderField = "NAME"
	// Order Workloads by status.
	WorkloadOrderFieldStatus WorkloadOrderField = "STATUS"
	// Order Workloads by the name of the environment.
	WorkloadOrderFieldEnvironment WorkloadOrderField = "ENVIRONMENT"
	// Order Workloads by the deployment time.
	WorkloadOrderFieldDeploymentTime WorkloadOrderField = "DEPLOYMENT_TIME"
)

var AllWorkloadOrderField = []WorkloadOrderField{
	WorkloadOrderFieldName,
	WorkloadOrderFieldStatus,
	WorkloadOrderFieldEnvironment,
	WorkloadOrderFieldDeploymentTime,
}

func (e WorkloadOrderField) IsValid() bool {
	switch e {
	case WorkloadOrderFieldName, WorkloadOrderFieldStatus, WorkloadOrderFieldEnvironment, WorkloadOrderFieldDeploymentTime:
		return true
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
