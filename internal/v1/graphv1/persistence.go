package graphv1

import (
	"context"

	"github.com/nais/api/internal/v1/workload"
	"github.com/nais/api/internal/v1/workload/application"
	"github.com/nais/api/internal/v1/workload/job"

	"github.com/nais/api/internal/slug"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Resolver) workload(ctx context.Context, ownerReference *metav1.OwnerReference, teamSlug slug.Slug, environmentName string) (workload.Workload, error) {
	if ownerReference == nil {
		return nil, nil
	}

	// TODO: Add support for "Unknown" kind, which will try app/job first, and the other second

	switch ownerReference.Kind {
	case "Naisjob":
		return job.Get(ctx, teamSlug, environmentName, ownerReference.Name)
	case "Application":
		return application.Get(ctx, teamSlug, environmentName, ownerReference.Name)
	default:
		r.log.WithField("kind", ownerReference.Kind).Warnf("Unsupported owner reference kind")
	}

	return nil, nil
}
