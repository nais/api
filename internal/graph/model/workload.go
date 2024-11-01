package model

import (
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type WorkloadBase struct {
	ID          scalar.Ident        `json:"id"`
	Name        string              `json:"name"`
	Image       string              `json:"image"`
	Env         Env                 `json:"env"`
	Status      WorkloadStatus      `json:"status"`
	Utilization AppUtilization      `json:"utilization"`
	GQLVars     WorkloadBaseGQLVars `json:"-"`
}

func (WorkloadBase) IsWorkload() {}

var _ Workload = (*WorkloadBase)(nil)

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

type AppUtilization struct {
	GQLVars AppGQLVars `json:"-"`
}

type AppGQLVars struct {
	TeamSlug slug.Slug
	AppName  string
	Env      string
}

type WorkloadBaseGQLVars struct {
	Spec        WorkloadSpec
	SecretNames []string
	Team        slug.Slug
}
