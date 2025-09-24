package status

import (
	"context"

	"github.com/nais/api/internal/vulnerability"
	"github.com/nais/api/internal/workload"
)

type checkVulnerabilities struct{}

func (checkVulnerabilities) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	summary, _, err := vulnerability.GetImageVulnerabilitySummaryAndWorkloadReferences(ctx, w.GetImageString())
	if err != nil {
		return nil, WorkloadStateUnknown
	}

	hasSBOM, err := vulnerability.GetImageHasSBOM(ctx, w.GetImageString())
	if err != nil {
		return nil, WorkloadStateUnknown
	}

	if summary == nil || !hasSBOM {
		return []WorkloadStatusError{&WorkloadStatusMissingSBOM{Level: WorkloadStatusErrorLevelTodo}}, WorkloadStateNais
	}

	if summary.Critical > 0 || summary.RiskScore >= 100 {
		return []WorkloadStatusError{
			&WorkloadStatusVulnerable{
				Level:   WorkloadStatusErrorLevelWarning,
				Summary: summary,
			},
		}, WorkloadStateNotNais
	}

	return nil, WorkloadStateNais
}

func (checkVulnerabilities) Supports(w workload.Workload) bool {
	return true
}
