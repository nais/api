package maintenance

import (
	"fmt"

	"github.com/nais/api/internal/graph/ident"
)

type identType int

const (
	identMaintenance identType = iota
)

func init() {
	// implements node pattern, e.g. retrieves a node by its identifier
	ident.RegisterIdentType(identMaintenance, "MAI", getVulnerabilityByIdent)
}

func parseWorkloadVulnerabilitySummaryIdent(id ident.Ident) (WorkloadReference, error) {
	parts := id.Parts()
	if len(parts) != 4 {
		return WorkloadReference{}, fmt.Errorf("invalid maintenance summary ident")
	}

	return WorkloadReference{
		Environment:  parts[0],
		Team:         parts[1],
		WorkloadType: parts[2],
		Name:         parts[3],
	}, nil
}
