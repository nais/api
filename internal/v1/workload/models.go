package workload

import (
	"strings"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
)

type Workload interface {
	modelv1.Node
	IsWorkload()
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
