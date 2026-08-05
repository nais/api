package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/issue"
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
