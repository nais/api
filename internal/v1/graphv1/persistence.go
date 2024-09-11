package graphv1

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/persistence"
	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"
)

func (r *Resolver) workload(ctx context.Context, workloadReference *persistence.WorkloadReference, teamSlug slug.Slug, environmentName string) (workload.Workload, error) {
	if workloadReference == nil {
		return nil, nil
	}

	switch workloadReference.Type {
	case persistence.WorkloadTypeJob:
		return job.Get(ctx, teamSlug, environmentName, workloadReference.Name)
	case persistence.WorkloadTypeApplication:
		return application.Get(ctx, teamSlug, environmentName, workloadReference.Name)
	default:
		return nil, fmt.Errorf("unsupported workload reference kind: %v", workloadReference.Type)
	}
}
