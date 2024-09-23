package graphv1

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func getWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (workload.Workload, error) {
	app, _ := application.Get(ctx, teamSlug, environmentName, workloadName)
	if app != nil {
		return app, nil
	}

	return job.Get(ctx, teamSlug, environmentName, workloadName)
}
