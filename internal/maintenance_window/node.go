package maintenancewindow

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identVulnerability identType = iota
	identWorkloadVulnerabilitySummary
)

func init() {
	// implements node pattern, e.g. retrieves a node by its identifier
	ident.RegisterIdentType(identVulnerability, "VUL", getVulnerabilityByIdent)
	ident.RegisterIdentType(identWorkloadVulnerabilitySummary, "WVS", getWorkloadVulnerabilitySummaryByIdent)
}

func newWorkloadVulnerabilitySummaryIdent(w WorkloadReference) ident.Ident {
	return ident.NewIdent(
		identWorkloadVulnerabilitySummary,
		w.Environment,
		w.Team,
		w.WorkloadType,
		w.Name,
	)
}

func parseWorkloadVulnerabilitySummaryIdent(id ident.Ident) (WorkloadReference, error) {
	parts := id.Parts()
	if len(parts) != 4 {
		return WorkloadReference{}, fmt.Errorf("invalid workload vulnerability summary ident")
	}

	return WorkloadReference{
		Environment:  parts[0],
		Team:         parts[1],
		WorkloadType: parts[2],
		Name:         parts[3],
	}, nil
}
