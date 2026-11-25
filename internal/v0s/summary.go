package v0s

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/workload"
	protoapi "github.com/nais/v0s/api/v0s/v1"
	"google.golang.org/protobuf/proto"
)

func selectionSummary(ctx context.Context, l *loaders, s scope, filter *protoapi.FindingFilter, current map[workloadKey]*protoapi.Workload, selected []workload.Workload) (*VulnerabilitySelectionSummary, error) {
	if err := requireRead(ctx); err != nil {
		return nil, err
	}
	var total int64
	if selected == nil {
		inventory, err := l.inventory.Load(ctx, struct{}{})
		if err != nil {
			return nil, err
		}
		for key := range inventory {
			if s.matches(key) {
				total++
			}
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
	coverageCounts := &workload.VulnerabilityCoverageCounts{
		TotalWorkloads: int(total), Scanned: int(c.GetScannedCount()),
		Pending: int(c.GetPendingCount() + total - c.GetTotalWorkloadCount()),
		NoSbom:  int(c.GetNoSbomCount()), Failed: int(c.GetFailedCount()), UnresolvedImage: int(c.GetUnresolvedImageCount()),
	}
	if selected != nil {
		coverageCounts, err = selectedWorkloadCoverage(selected, current)
		if err != nil {
			return nil, err
		}
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
		Coverage: coverageCounts,
	}, nil
}

func selectedWorkloadCoverage(selected []workload.Workload, current map[workloadKey]*protoapi.Workload) (*workload.VulnerabilityCoverageCounts, error) {
	counts := &workload.VulnerabilityCoverageCounts{}
	seen := make(map[workloadIdentity]bool, len(selected))
	for _, item := range selected {
		key := overviewKey(item, "")
		if seen[key.workloadIdentity] {
			continue
		}
		seen[key.workloadIdentity] = true
		row := current[key.workload]
		if key.image == "" || row.GetImageReference() != key.image {
			row = nil
		}
		scan, err := coverage(row)
		if err != nil {
			return nil, err
		}
		counts.TotalWorkloads++
		switch scan.Status {
		case VulnerabilityCoverageStatusScanned:
			counts.Scanned++
		case VulnerabilityCoverageStatusNoSbom:
			counts.NoSbom++
		case VulnerabilityCoverageStatusFailed:
			counts.Failed++
		case VulnerabilityCoverageStatusUnresolvedImage:
			counts.UnresolvedImage++
		case VulnerabilityCoverageStatusPending:
			counts.Pending++
		}
	}
	return counts, nil
}
