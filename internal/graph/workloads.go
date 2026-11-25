package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v0s"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func newWorkloadConnection(scope v0s.Scope, all, pageItems []workload.Workload, page *pagination.Pagination, coverage *workload.VulnerabilityCoverageCounts) *workload.WorkloadConnection {
	return &workload.WorkloadConnection{
		Connection: *pagination.NewConnection(pageItems, page, len(all)),
		Facets: &workload.WorkloadFacets{LoadCoverage: func(ctx context.Context) (*workload.VulnerabilityCoverageCounts, error) {
			if coverage != nil {
				return coverage, nil
			}
			statuses, err := v0s.WorkloadCoverageStatuses(ctx, scope, all)
			if err != nil {
				return nil, err
			}
			return countWorkloadCoverage(statuses), nil
		}},
	}
}

func countWorkloadCoverage(statuses []v0s.VulnerabilityCoverageStatus) *workload.VulnerabilityCoverageCounts {
	counts := make(map[v0s.VulnerabilityCoverageStatus]int)
	for _, status := range statuses {
		counts[status]++
	}
	return &workload.VulnerabilityCoverageCounts{
		TotalWorkloads: len(statuses), Scanned: counts[v0s.VulnerabilityCoverageStatusScanned],
		Pending: counts[v0s.VulnerabilityCoverageStatusPending], NoSbom: counts[v0s.VulnerabilityCoverageStatusNoSbom],
		Failed: counts[v0s.VulnerabilityCoverageStatusFailed], UnresolvedImage: counts[v0s.VulnerabilityCoverageStatusUnresolvedImage],
	}
}

func selectTeamWorkloads(ctx context.Context, team slug.Slug, filter *workload.TeamWorkloadsFilter) []workload.Workload {
	apps := application.ListAllForTeam(ctx, team, nil, nil)
	jobs := job.ListAllForTeam(ctx, team, nil, nil)
	items := make([]workload.Workload, 0, len(apps)+len(jobs))
	for _, app := range apps {
		items = append(items, app)
	}
	for _, j := range jobs {
		items = append(items, j)
	}
	return workload.SortFilter.Filter(ctx, items, filter)
}

// tryWorkload attempts to find a workload by name, first as an application, then as a job.
func tryWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (workload.Workload, error) {
	app, _ := application.Get(ctx, teamSlug, environmentName, workloadName)
	if app != nil {
		return app, nil
	}

	j, err := job.Get(ctx, teamSlug, environmentName, workloadName)
	if err != nil {
		// Returning j here would wrap a nil pointer in a non-nil interface.
		return nil, err
	}
	return j, nil
}

// workloadOrNil resolves a workload reference that is allowed to dangle, such as a service account
// binding.
func workloadOrNil(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) workload.Workload {
	w, _ := tryWorkload(ctx, teamSlug, environmentName, workloadName)
	return w
}

func getWorkload(ctx context.Context, workloadReference *workload.Reference, teamSlug slug.Slug, environmentName string) (workload.Workload, error) {
	if workloadReference == nil {
		return nil, nil
	}

	switch workloadReference.Type {
	case workload.TypeJob:
		return job.Get(ctx, teamSlug, environmentName, workloadReference.Name)
	case workload.TypeApplication:
		return application.Get(ctx, teamSlug, environmentName, workloadReference.Name)
	default:
		return nil, fmt.Errorf("unsupported workload reference kind: %v", workloadReference.Type)
	}
}

func getWorkloadByResourceType(ctx context.Context, teamSlug slug.Slug, environmentName, resourceName string, resourceType issue.ResourceType) (workload.Workload, error) {
	switch resourceType {
	case issue.ResourceTypeApplication:
		return application.Get(ctx, teamSlug, environmentName, resourceName)
	case issue.ResourceTypeJob:
		return job.Get(ctx, teamSlug, environmentName, resourceName)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}
