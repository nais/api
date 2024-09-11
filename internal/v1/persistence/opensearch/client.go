package opensearch

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/persistence"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

type client struct {
	watcher *watcher.Watcher[*OpenSearch]
}

func openSearchNamer(teamSlug slug.Slug, instanceName string) string {
	return "opensearch-" + teamSlug.String() + "-" + instanceName
}

func (c client) getAccessForApplications(ctx context.Context, environmentName, openSearchName string, teamSlug slug.Slug) ([]*OpenSearchAccess, error) {
	access := make([]*OpenSearchAccess, 0)
	workloads := application.ListAllForTeam(ctx, teamSlug)

	for _, w := range workloads {

		if w.Spec.OpenSearch == nil {
			continue
		}

		if openSearchNamer(teamSlug, w.Spec.OpenSearch.Instance) == openSearchName {
			access = append(access, &OpenSearchAccess{
				Access:          w.Spec.OpenSearch.Access,
				TeamSlug:        teamSlug,
				EnvironmentName: environmentName,
				WorkloadReference: &persistence.WorkloadReference{
					Name: w.Name,
					Type: persistence.WorkloadTypeApplication,
				},
			})
		}
	}

	return access, nil
}

func (c client) getAccessForJobs(ctx context.Context, environmentName, openSearchName string, teamSlug slug.Slug) ([]*OpenSearchAccess, error) {
	access := make([]*OpenSearchAccess, 0)

	workloads := job.ListAllForTeam(ctx, teamSlug)
	for _, w := range workloads {
		if w.Spec.OpenSearch == nil {
			continue
		}

		if openSearchNamer(teamSlug, w.Spec.OpenSearch.Instance) == openSearchName {
			access = append(access, &OpenSearchAccess{
				Access:          w.Spec.OpenSearch.Access,
				TeamSlug:        teamSlug,
				EnvironmentName: environmentName,
				WorkloadReference: &persistence.WorkloadReference{
					Name: w.Name,
					Type: persistence.WorkloadTypeJob,
				},
			})
		}
	}

	return access, nil
}
