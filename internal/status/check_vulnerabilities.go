package status

import (
	"context"

	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload"
)

type checkVulnerabilities struct{}

func (checkVulnerabilities) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	md, err := vulnerability.GetImageMetadata(ctx, w.GetImageString())
	if err != nil {
		return nil, WorkloadStateUnknown
	}

	if md.Summary == nil || !md.HasSBOM {
		return []WorkloadStatusError{&WorkloadStatusMissingSBOM{Level: WorkloadStatusErrorLevelTodo}}, WorkloadStateNais
	}

	if md.Summary.Critical > 0 || md.Summary.RiskScore >= 100 {
		return []WorkloadStatusError{
			&WorkloadStatusVulnerable{
				Level:   WorkloadStatusErrorLevelWarning,
				Summary: md.Summary,
			},
		}, WorkloadStateNotNais
	}

	return nil, WorkloadStateNais
}

func (checkVulnerabilities) Supports(w workload.Workload) bool {
	return true
}
