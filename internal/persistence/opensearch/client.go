package opensearch

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

type client struct{}

func instanceNamer(teamSlug slug.Slug, instanceName string) string {
	return namePrefix(teamSlug) + instanceName
}

func namePrefix(teamSlug slug.Slug) string {
	return "opensearch-" + teamSlug.String() + "-"
}

func (c client) getAccessForApplications(ctx context.Context, environmentName, openSearchName string, teamSlug slug.Slug) ([]*OpenSearchAccess, error) {
	access := make([]*OpenSearchAccess, 0)
	workloads := application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)

	for _, w := range workloads {

		if w.Spec.OpenSearch == nil {
			continue
		}

		if instanceNamer(teamSlug, w.Spec.OpenSearch.Instance) == openSearchName {
			access = append(access, &OpenSearchAccess{
				Access:          w.Spec.OpenSearch.Access,
				TeamSlug:        teamSlug,
				EnvironmentName: environmentName,
				WorkloadReference: &workload.Reference{
					Name: w.Name,
					Type: workload.TypeApplication,
				},
			})
		}
	}

	return access, nil
}

func (c client) getAccessForJobs(ctx context.Context, environmentName, openSearchName string, teamSlug slug.Slug) ([]*OpenSearchAccess, error) {
	access := make([]*OpenSearchAccess, 0)

	workloads := job.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	for _, w := range workloads {
		if w.Spec.OpenSearch == nil {
			continue
		}

		if instanceNamer(teamSlug, w.Spec.OpenSearch.Instance) == openSearchName {
			access = append(access, &OpenSearchAccess{
				Access:          w.Spec.OpenSearch.Access,
				TeamSlug:        teamSlug,
				EnvironmentName: environmentName,
				WorkloadReference: &workload.Reference{
					Name: w.Name,
					Type: workload.TypeJob,
				},
			})
		}
	}

	return access, nil
}
