package model

import "github.com/nais/api/internal/graph/scalar"

type ImageDetails struct {
	ID         scalar.Ident               `json:"id"`
	ProjectID  string                     `json:"projectId"`
	Name       string                     `json:"name"`
	Version    string                     `json:"version"`
	Digest     string                     `json:"digest"`
	Rekor      Rekor                      `json:"rekor"`
	Summary    *ImageVulnerabilitySummary `json:"summary"`
	HasSbom    bool                       `json:"hasSbom"`
	ProjectURL string                     `json:"projectUrl"`

	GQLVars ImageDetailsGQLVars `json:"-"`
}

type AnalysisTrail struct {
	ID    scalar.Ident `json:"id"`
	State string       `json:"state"`
	// Comments     AnalysisCommentList `json:"comments"`
	IsSuppressed bool `json:"isSuppressed"`

	GQLVars AnalysisTrailGQLVars `json:"-"`
}

type AnalysisTrailGQLVars struct {
	Comments []*AnalysisComment `json:"comments"`
}

type ImageDetailsGQLVars struct {
	WorkloadReferences []*WorkloadReference
}

type WorkloadReference struct {
	ID           scalar.Ident `json:"id"`
	Name         string       `json:"name"`
	Team         string       `json:"team"`
	WorkloadType string       `json:"workloadType"`
	Environment  string       `json:"environment"`
}
