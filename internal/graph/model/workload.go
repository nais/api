package model

import (
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type WorkloadBase struct {
	ID           scalar.Ident        `json:"id"`
	Name         string              `json:"name"`
	Image        string              `json:"image"`
	ImageDetails ImageDetails        `json:"imageDetails"`
	DeployInfo   DeployInfo          `json:"deployInfo"`
	Env          Env                 `json:"env"`
	AccessPolicy AccessPolicy        `json:"accessPolicy"`
	Status       WorkloadStatus      `json:"status"`
	Authz        []Authz             `json:"authz"`
	Variables    []*Variable         `json:"variables"`
	Resources    Resources           `json:"resources"`
	GQLVars      WorkloadBaseGQLVars `json:"-"`
}

func (WorkloadBase) IsWorkload()                     {}
func (w WorkloadBase) GetID() scalar.Ident           { return w.ID }
func (w WorkloadBase) GetName() string               { return w.Name }
func (w WorkloadBase) GetImage() string              { return w.Image }
func (w WorkloadBase) GetImageDetails() ImageDetails { return w.ImageDetails }
func (w WorkloadBase) GetDeployInfo() DeployInfo     { return w.DeployInfo }
func (w WorkloadBase) GetEnv() Env                   { return w.Env }
func (w WorkloadBase) GetAccessPolicy() AccessPolicy { return w.AccessPolicy }
func (w WorkloadBase) GetStatus() WorkloadStatus     { return w.Status }
func (w WorkloadBase) GetAuthz() []Authz             { return w.Authz }
func (w WorkloadBase) GetVariables() []*Variable     { return w.Variables }
func (w WorkloadBase) GetResources() Resources       { return w.Resources }

type WorkloadStatus struct {
	State  State        `json:"state"`
	Errors []StateError `json:"errors"`
}

type WorkloadSpec struct {
	GCP        *nais_io_v1.GCP
	Kafka      *nais_io_v1.Kafka
	OpenSearch *nais_io_v1.OpenSearch
	Redis      []nais_io_v1.Redis
}

type WorkloadBaseGQLVars struct {
	Spec        WorkloadSpec
	SecretNames []string
	Team        slug.Slug
}
