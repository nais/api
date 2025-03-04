package status

// This check is temporarily

/*
import (
	"context"
	"strings"

	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/netpol"
)
type checkNetpol struct{}

func (checkNetpol) Run(ctx context.Context, w workload.Workload) ([]WorkloadStatusError, WorkloadState) {
	if strings.Contains(w.GetEnvironmentName(), "-fss") {
		return nil, WorkloadStateNais
	}

	policy := netpol.ListForWorkload(ctx, w.GetTeamSlug(), w.GetEnvironmentName(), w.GetName(), w.GetAccessPolicy())

	ret := []WorkloadStatusError{}
	for _, p := range policy.Inbound.Rules {
		if isNotZeroTrust(w.GetEnvironmentName(), p) {
			continue
		}
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
		if isNotZeroTrust(w.GetEnvironmentName(), p) {
			continue
		}
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

func isNotZeroTrust(env string, rule *netpol.NetworkPolicyRule) bool {
	if strings.Contains(env, "-fss") {
		return true
	}

	if strings.Contains(rule.Cluster, "-fss") {
		return true
	}

	if rule.TargetTeamSlug == "nais-system" {
		return true
	}

	if strings.Contains(rule.Cluster, "-external") {
		return true
	}

	return false
}
*/
