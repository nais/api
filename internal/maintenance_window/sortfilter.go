package maintenancewindow

import (
	"context"

	"github.com/nais/api/internal/graph/sortfilter"
	"github.com/nais/api/internal/workload"
	"github.com/nais/v13s/pkg/api/vulnerabilities"
)

var SortFilterImageVulnerabilities = map[ImageVulnerabilityOrderField]vulnerabilities.OrderByField{
	"SEVERITY":   vulnerabilities.OrderBySeverity,
	"IDENTIFIER": vulnerabilities.OrderByCveId,
	"PACKAGE":    vulnerabilities.OrderByPackage,
	"STATE":      vulnerabilities.OrderByReason,
	"SUPPRESSED": vulnerabilities.OrderBySuppressed,
}

var SortFilterWorkloadSummaries = map[VulnerabilitySummaryOrderByField]vulnerabilities.OrderByField{
	"NAME":                              vulnerabilities.OrderByWorkload,
	"ENVIRONMENT":                       vulnerabilities.OrderByCluster,
	"VULNERABILITY_RISK_SCORE":          vulnerabilities.OrderByRiskScore,
	"VULNERABILITY_SEVERITY_CRITICAL":   vulnerabilities.OrderByCritical,
	"VULNERABILITY_SEVERITY_HIGH":       vulnerabilities.OrderByHigh,
	"VULNERABILITY_SEVERITY_MEDIUM":     vulnerabilities.OrderByMedium,
	"VULNERABILITY_SEVERITY_LOW":        vulnerabilities.OrderByLow,
	"VULNERABILITY_SEVERITY_UNASSIGNED": vulnerabilities.OrderByUnassigned,
}

func init() {
	workloadInit()
}

func workloadInit() {
	summarySorter := func(fn func(sum *ImageVulnerabilitySummary) int) sortfilter.ConcurrentSortFunc[workload.Workload] {
		return func(ctx context.Context, a workload.Workload) int {
			ref, err := GetImageMetadata(ctx, a.GetImageString())
			if err != nil {
				return -1
			}

			if ref == nil || ref.Summary == nil {
				return -1
			}

			return fn(ref.Summary)
		}
	}

	workload.SortFilter.RegisterConcurrentSort("VULNERABILITY_RISK_SCORE", summarySorter(func(sum *ImageVulnerabilitySummary) int {
		return sum.RiskScore
	}), "NAME", "ENVIRONMENT")
	workload.SortFilter.RegisterConcurrentSort("VULNERABILITY_SEVERITY_CRITICAL", summarySorter(func(sum *ImageVulnerabilitySummary) int {
		return sum.Critical
	}), "NAME", "ENVIRONMENT")
	workload.SortFilter.RegisterConcurrentSort("VULNERABILITY_SEVERITY_HIGH", summarySorter(func(sum *ImageVulnerabilitySummary) int {
		return sum.High
	}), "NAME", "ENVIRONMENT")
	workload.SortFilter.RegisterConcurrentSort("VULNERABILITY_SEVERITY_MEDIUM", summarySorter(func(sum *ImageVulnerabilitySummary) int {
		return sum.Medium
	}), "NAME", "ENVIRONMENT")
	workload.SortFilter.RegisterConcurrentSort("VULNERABILITY_SEVERITY_LOW", summarySorter(func(sum *ImageVulnerabilitySummary) int {
		return sum.Low
	}), "NAME", "ENVIRONMENT")
	workload.SortFilter.RegisterConcurrentSort("VULNERABILITY_SEVERITY_UNASSIGNED", summarySorter(func(sum *ImageVulnerabilitySummary) int {
		return sum.Unassigned
	}), "NAME", "ENVIRONMENT")
}
