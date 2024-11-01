package model

import (
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type App struct {
	WorkloadBase
	Ingresses []string `json:"ingresses"`
}

func (App) IsSearchNode() {}

var _ Workload = (*App)(nil)

type Instance struct {
	ID       scalar.Ident  `json:"id"`
	Name     string        `json:"name"`
	State    InstanceState `json:"state"`
	Message  string        `json:"message"`
	Image    string        `json:"image"`
	Restarts int           `json:"restarts"`
	Created  time.Time     `json:"created"`

	GQLVars InstanceGQLVars `json:"-"`
}

type InstanceGQLVars struct {
	Env     string
	Team    slug.Slug
	AppName string
}

type AppUtilizationData struct {
	Env       string    `json:"env"`
	Requested float64   `json:"requested"`
	Used      float64   `json:"used"`
	AppName   string    `json:"-"`
	TeamSlug  slug.Slug `json:"-"`
}
