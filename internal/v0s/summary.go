package v0s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/workload"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/proto"
)

func selectionSummary(ctx context.Context, l *loaders, s scope, filter *protoapi.FindingFilter, current map[workloadKey]*protoapi.Workload) (*VulnerabilitySelectionSummary, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	inventory, err := l.inventory.Load(ctx, struct{}{})
	if err != nil {
		return nil, err
	}
	var total int64
	for key := range inventory {
		if s.matches(key) {
			total++
		}
	}
	f := proto.Clone(filter).(*protoapi.FindingFilter)
	f.Team, f.Environment, f.Workload = "", "", ""
	resp, err := l.manager.Vulnerabilities.GetSummary(ctx, &protoapi.GetSummaryRequest{
		Team: s.team, Environment: s.environment, Workload: s.workload,
		FindingFilter: f, WorkloadSelector: workloadSelector(current),
	})
	if err != nil {
		return nil, backendError("get v0s vulnerability summary", err)
	}
	if resp == nil || resp.GetSeverityCounts() == nil || resp.GetCoverage() == nil {
		return nil, fmt.Errorf("v0s vulnerability summary is missing counts or coverage")
	}
	c := resp.GetCoverage()
	if c.GetTotalWorkloadCount() != int64(len(current)) ||
		c.GetScannedCount()+c.GetPendingCount()+c.GetNoSbomCount()+c.GetFailedCount()+c.GetUnresolvedImageCount() != c.GetTotalWorkloadCount() {
		return nil, fmt.Errorf("v0s vulnerability summary coverage changed; retry the request")
	}
	severities := resp.GetSeverityCounts()
	return &VulnerabilitySelectionSummary{
		FindingCount: resp.GetFindingCount(), UniqueVulnerabilityCount: resp.GetUniqueVulnerabilityCount(),
		AffectedWorkloadCount:            resp.GetAffectedWorkloadCount(),
		KnownExploitedVulnerabilityCount: resp.GetUniqueKevVulnerabilityCount(),
		KnownExploitedWorkloadCount:      resp.GetKevAffectedWorkloadCount(),
		CountsBySeverity: &VulnerabilitySeverityCounts{
			Critical: severities.GetCritical(), High: severities.GetHigh(), Medium: severities.GetMedium(),
			Low: severities.GetLow(), Unknown: severities.GetUnknown(),
		},
		Coverage: &workload.VulnerabilityCoverageCounts{
			TotalWorkloads: int(total), Scanned: int(c.GetScannedCount()),
			Pending: int(c.GetPendingCount() + total - c.GetTotalWorkloadCount()),
			NoSbom:  int(c.GetNoSbomCount()), Failed: int(c.GetFailedCount()), UnresolvedImage: int(c.GetUnresolvedImageCount()),
		},
	}, nil
}
