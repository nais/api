package valkey

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

type client struct {
	watcher *watcher.Watcher[*Valkey]
}

func namePrefix(teamSlug slug.Slug) string {
	return "valkey-" + teamSlug.String() + "-"
}

func instanceNamer(teamSlug slug.Slug, instanceName string) string {
	return namePrefix(teamSlug) + instanceName
}

func (c client) getAccessForApplications(ctx context.Context, environmentName, valkeyName string, teamSlug slug.Slug) ([]*ValkeyAccess, error) {
	access := make([]*ValkeyAccess, 0)

	workloads := application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	for _, w := range workloads {
		for _, r := range w.Spec.Valkey {
			if instanceNamer(teamSlug, r.Instance) == valkeyName {
				access = append(access, &ValkeyAccess{
					Access:          r.Access,
					TeamSlug:        teamSlug,
					EnvironmentName: environmentName,
					WorkloadReference: &workload.Reference{
						Name: w.Name,
						Type: workload.TypeApplication,
					},
				})
			}
		}
	}

	return access, nil
}

func (c client) getAccessForJobs(ctx context.Context, environmentName, valkeyName string, teamSlug slug.Slug) ([]*ValkeyAccess, error) {
	access := make([]*ValkeyAccess, 0)

	workloads := job.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	for _, w := range workloads {
		for _, r := range w.Spec.Valkey {
			if instanceNamer(teamSlug, r.Instance) == valkeyName {
				access = append(access, &ValkeyAccess{
					Access:          r.Access,
					TeamSlug:        teamSlug,
					EnvironmentName: environmentName,
					WorkloadReference: &workload.Reference{
						Name: w.Name,
						Type: workload.TypeJob,
					},
				})
			}
		}
	}

	return access, nil
}
