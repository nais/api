package graph

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/issue"
	"github.com/nais/api/internal/resource"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
)

func getResourceByResourceType(ctx context.Context, teamSlug slug.Slug, environmentName, resourceName string, resourceType issue.ResourceType) (resource.Resource, error) {
	switch resourceType {
	case issue.ResourceTypeApplication:
		return application.Get(ctx, teamSlug, environmentName, resourceName)
	case issue.ResourceTypeJob:
		return job.Get(ctx, teamSlug, environmentName, resourceName)
	default:
		return nil, fmt.Errorf("unknown resource type: %s", resourceType)
	}
}
