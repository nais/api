package valkey

import (
	"context"
	"strings"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

type client struct{}

// NamePrefix returns the Kubernetes resource name prefix for Valkey instances
// belonging to the given team (e.g. "valkey-myteam-").
func NamePrefix(teamSlug slug.Slug) string {
	return "valkey-" + teamSlug.String() + "-"
}

func instanceNamer(teamSlug slug.Slug, instanceName string) string {
	return NamePrefix(teamSlug) + instanceName
}

// The aiven.io watcher keys on the prefixed name, the nais.io one on the bare name.
func fullyQualifiedName(teamSlug slug.Slug, name string) string {
	if strings.HasPrefix(name, NamePrefix(teamSlug)) {
		return name
	}
	return instanceNamer(teamSlug, name)
}

func (c client) getAccessForApplications(ctx context.Context, environmentName, valkeyName string, teamSlug slug.Slug) ([]*ValkeyAccess, error) {
	access := make([]*ValkeyAccess, 0)

	workloads := application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
	for _, w := range workloads {
		for _, r := range w.Spec.Valkey {
			if fullyQualifiedName(teamSlug, r.Instance) == valkeyName {
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
			if fullyQualifiedName(teamSlug, r.Instance) == valkeyName {
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
