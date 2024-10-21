package model

import "github.com/nais/api/internal/graph/scalar"

type ImageDetails struct {
	ID         scalar.Ident               `json:"id"`
	ProjectID  string                     `json:"projectId"`
	Name       string                     `json:"name"`
	Version    string                     `json:"version"`
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

func (w *WorkloadReference) ContainsEnv(envs []string) bool {
	for _, e := range envs {
		if w.Environment == e {
			return true
		}
	}
	return false
}

func (i ImageDetailsGQLVars) ContainsReference(env, team, workloadType, name string) bool {
	for _, r := range i.WorkloadReferences {
		if r.Matches(env, team, workloadType, name) {
			return true
		}
	}
	return false
}

func (i ImageDetailsGQLVars) GetWorkloadReference(env, team, workloadType, name string) *WorkloadReference {
	for _, r := range i.WorkloadReferences {
		if r.Matches(env, team, workloadType, name) {
			return r
		}
	}
	return nil
}

func (w *WorkloadReference) Matches(env, team, workloadType, name string) bool {
	return w.Environment == env && w.Team == team && w.WorkloadType == workloadType && w.Name == name
}
