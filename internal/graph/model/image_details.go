package model

import "github.com/nais/api/internal/graph/scalar"

type ImageDetails struct {
	ID                 scalar.Ident              `json:"id"`
	ProjectID          string                    `json:"projectId"`
	Name               string                    `json:"name"`
	Version            string                    `json:"version"`
	Digest             string                    `json:"digest"`
	Rekor              Rekor                     `json:"rekor"`
	Summary            ImageVulnerabilitySummary `json:"summary"`
	HasSbom            bool                      `json:"hasSbom"`
	Findings           FindingList               `json:"findings"`
	WorkloadReferences []*WorkloadReference      `json:"workloadReferences"`
}
