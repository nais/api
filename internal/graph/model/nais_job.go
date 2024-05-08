package model

import (
	"time"

	"github.com/nais/api/internal/graph/scalar"
	"github.com/nais/api/internal/slug"
)

type NaisJob struct {
	WorkloadBase
	Schedule    string `json:"schedule"`
	Completions int    `json:"completions"`
	Parallelism int    `json:"parallelism"`
	Retries     int    `json:"retries"`
}

func (NaisJob) IsSearchNode() {}
func (NaisJob) IsWorkload()   {}

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

type RunGQLVars struct {
	Env     string
	Team    slug.Slug
	NaisJob string
}
