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
	HasSbom            bool                 `json:"hasSbom"`
	WorkloadReferences []*WorkloadReference `json:"workloadReferences"`
}
