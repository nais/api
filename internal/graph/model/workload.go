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
	DeployInfo   DeployInfo          `json:"deployInfo"`
	Env          Env                 `json:"env"`
	AccessPolicy AccessPolicy        `json:"accessPolicy"`
	Status       WorkloadStatus      `json:"status"`
	Authz        []Authz             `json:"authz"`
	Variables    []*Variable         `json:"variables"`
	Resources    Resources           `json:"resources"`
	GQLVars      WorkloadBaseGQLVars `json:"-"`
}

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
