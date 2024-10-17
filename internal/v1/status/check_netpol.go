package status

import (
	"context"

	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/netpol"
)

type checkNetpol struct{}

func (checkNetpol) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	policy := netpol.ListForWorkload(ctx, w.GetTeamSlug(), w.GetEnvironmentName(), w.GetName(), w.GetAccessPolicy())

	ret := []WorkloadStatusError{}
	for _, p := range policy.Inbound.Rules {
		isAllowed := netpol.AllowsOutboundWorkload(ctx, p.TargetTeamSlug, p.EnvironmentName, p.TargetWorkloadName, p.TeamSlug, p.WorkloadName)
		if isAllowed {
			continue
		}

		ret = append(ret, &WorkloadStatusInboundNetwork{
			Level:  WorkloadStatusErrorLevelWarning,
			Policy: p,
		})
	}

	for _, p := range policy.Outbound.Rules {
		isAllowed := netpol.AllowsInboundWorkload(ctx, p.TargetTeamSlug, p.EnvironmentName, p.TargetWorkloadName, p.TeamSlug, p.WorkloadName)
		if isAllowed {
			continue
		}

		ret = append(ret, &WorkloadStatusOutboundNetwork{
			Level:  WorkloadStatusErrorLevelWarning,
			Policy: p,
		})
	}

	if len(ret) == 0 {
		return nil, WorkloadStateNais
	}

	return ret, WorkloadStateNotNais
}

func (checkNetpol) Supports(w workload.Workload) bool {
	return true
}
