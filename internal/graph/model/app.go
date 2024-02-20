package model

import (
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type App struct {
	ID           scalar.Ident `json:"id"`
	Name         string       `json:"name"`
	Image        string       `json:"image"`
	DeployInfo   DeployInfo   `json:"deployInfo"`
	Env          Env          `json:"env"`
	Ingresses    []string     `json:"ingresses"`
	AccessPolicy AccessPolicy `json:"accessPolicy"`
	Resources    Resources    `json:"resources"`
	AutoScaling  AutoScaling  `json:"autoScaling"`
	Storage      []Storage    `json:"storage"`
	Variables    []*Variable  `json:"variables"`
	Authz        []Authz      `json:"authz"`
	AppState     AppState     `json:"appState"`
	GQLVars      AppGQLVars   `json:"-"`
}

func (App) IsSearchNode() {}

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

type AppGQLVars struct {
	Team slug.Slug
}

type InstanceGQLVars struct {
	Env     string
	Team    slug.Slug
	AppName string
}
