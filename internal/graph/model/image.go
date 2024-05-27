package model

import "github.com/nais/api/internal/graph/scalar"

type Image struct {
	ID                 scalar.Ident         `json:"id"`
	ProjectID          string               `json:"projectId"`
	Name               string               `json:"name"`
	Version            string               `json:"version"`
	Digest             string               `json:"digest"`
	RekorID            string               `json:"rekorId"`
	Summary            VulnerabilitySummary `json:"summary"`
	WorkloadReferences []*WorkloadReference `json:"workloadReferences"`
}

type AnalysisTrail struct {
	State        string     `json:"state"`
	Comments     []*Comment `json:"comments"`
	IsSuppressed bool       `json:"isSuppressed"`
}
