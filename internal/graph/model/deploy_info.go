package model

import "time"

type DeployInfo struct {
	Deployer  string     `json:"deployer"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
	CommitSha string     `json:"commitSha"`
	URL       string     `json:"url"`

	GQLVars DeployInfoGQLVars `json:"-"`
}
