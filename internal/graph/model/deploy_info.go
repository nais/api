package model

import (
	"time"

	"github.com/nais/api/internal/slug"
)

type DeployInfo struct {
	Deployer  string            `json:"deployer"`
	Timestamp *time.Time        `json:"timestamp,omitempty"`
	CommitSha string            `json:"commitSha"`
	URL       string            `json:"url"`
	GQLVars   DeployInfoGQLVars `json:"-"`
}

type DeployInfoGQLVars struct {
	App  string
	Job  string
	Env  string
	Team slug.Slug
}
