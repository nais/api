package model

import "github.com/nais/api/internal/graph/scalar"

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

	GQLVars AppGQLVars `json:"-"`
}

func (App) IsSearchNode() {}
