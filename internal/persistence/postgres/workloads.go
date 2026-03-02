package postgres

import (
	"cmp"
	"context"
	"slices"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func WorkloadsForInstance(ctx context.Context, teamSlug slug.Slug, environmentName, clusterName string) ([]workload.Workload, error) {
	entry := fromContext(ctx).workloadsEntry(teamSlug.String(), environmentName)
	entry.once.Do(func() {
		apps := application.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)
		jobs := job.ListAllForTeamInEnvironment(ctx, teamSlug, environmentName)

		index := make(map[string][]workload.Workload)
		for _, app := range apps {
			if app.Spec != nil && app.Spec.Postgres != nil && app.Spec.Postgres.ClusterName != "" {
				index[app.Spec.Postgres.ClusterName] = append(index[app.Spec.Postgres.ClusterName], app)
			}
		}

		for _, j := range jobs {
			if j.Spec != nil && j.Spec.Postgres != nil && j.Spec.Postgres.ClusterName != "" {
				index[j.Spec.Postgres.ClusterName] = append(index[j.Spec.Postgres.ClusterName], j)
			}
		}

		for postgresClusterName, workloads := range index {
			slices.SortFunc(workloads, func(a, b workload.Workload) int {
				if a.GetName() != b.GetName() {
					return cmp.Compare(a.GetName(), b.GetName())
				}

				return cmp.Compare(a.GetType().String(), b.GetType().String())
			})
			index[postgresClusterName] = workloads
		}

		entry.workloads = index
	})

	ret := entry.workloads[clusterName]
	if ret == nil {
		return []workload.Workload{}, nil
	}

	return ret, nil
}
