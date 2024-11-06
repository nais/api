package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

// tryWorkload attempts to find a workload by name, first as an application, then as a job.
func tryWorkload(ctx context.Context, teamSlug slug.Slug, environmentName, workloadName string) (workload.Workload, error) {
	app, _ := application.Get(ctx, teamSlug, environmentName, workloadName)
	if app != nil {
		return app, nil
	}

	return job.Get(ctx, teamSlug, environmentName, workloadName)
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
