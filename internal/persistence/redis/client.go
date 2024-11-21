package redis

import (
	"context"

	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

type client struct {
	watcher *watcher.Watcher[*RedisInstance]
}

func redisInstanceNamer(teamSlug slug.Slug, instanceName string) string {
	return "redis-" + teamSlug.String() + "-" + instanceName
}

func (c client) getAccessForApplications(ctx context.Context, environmentName, redisInstanceName string, teamSlug slug.Slug) ([]*RedisInstanceAccess, error) {
	access := make([]*RedisInstanceAccess, 0)

	workloads := application.ListAllForTeam(ctx, teamSlug)
	for _, w := range workloads {
		for _, r := range w.Spec.Redis {
			if redisInstanceNamer(teamSlug, r.Instance) == redisInstanceName {
				access = append(access, &RedisInstanceAccess{
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

func (c client) getAccessForJobs(ctx context.Context, environmentName, redisInstanceName string, teamSlug slug.Slug) ([]*RedisInstanceAccess, error) {
	access := make([]*RedisInstanceAccess, 0)

	workloads := job.ListAllForTeam(ctx, teamSlug)
	for _, w := range workloads {
		for _, r := range w.Spec.Redis {
			if redisInstanceNamer(teamSlug, r.Instance) == redisInstanceName {
				access = append(access, &RedisInstanceAccess{
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
