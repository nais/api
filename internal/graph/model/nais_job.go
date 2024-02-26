package model

import (
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type NaisJob struct {
	ID           scalar.Ident   `json:"id"`
	AccessPolicy AccessPolicy   `json:"accessPolicy"`
	DeployInfo   DeployInfo     `json:"deployInfo"`
	Env          Env            `json:"env"`
	Image        string         `json:"image"`
	Name         string         `json:"name"`
	Resources    Resources      `json:"resources"`
	Schedule     string         `json:"schedule"`
	Storage      []Storage      `json:"storage"`
	Authz        []Authz        `json:"authz"`
	Completions  int            `json:"completions"`
	Parallelism  int            `json:"parallelism"`
	Retries      int            `json:"retries"`
	JobState     JobState       `json:"jobState"`
	GQLVars      NaisJobGQLVars `json:"-"`
}

func (NaisJob) IsSearchNode() {}

type Run struct {
	ID             scalar.Ident `json:"id"`
	Name           string       `json:"name"`
	PodNames       []string     `json:"podNames"`
	StartTime      *time.Time   `json:"startTime,omitempty"`
	CompletionTime *time.Time   `json:"completionTime,omitempty"`
	Duration       string       `json:"duration"`
	Image          string       `json:"image"`
	Message        string       `json:"message"`
	Failed         bool         `json:"failed"`
	GQLVars        RunGQLVars   `json:"-"`
}

type NaisJobGQLVars struct {
	SecretNames []string
	Team        slug.Slug
}

type RunGQLVars struct {
	Env     string
	Team    slug.Slug
	NaisJob string
}
