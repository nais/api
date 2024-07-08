package graphv1

import (
	"context"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/application"
	"github.com/nais/api/internal/workload/job"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Resolver) workload(ctx context.Context, ownerReference *metav1.OwnerReference, teamSlug slug.Slug, environmentName string) (workload.Workload, error) {
	if ownerReference == nil {
		return nil, nil
	}

	switch ownerReference.Kind {
	case "Naisjob":
		return job.Get(ctx, teamSlug, ownerReference.Name, environmentName)
	case "Application":
		return application.Get(ctx, teamSlug, ownerReference.Name, environmentName)
	default:
		r.log.WithField("kind", ownerReference.Kind).Warnf("Unsupported owner reference kind")
	}

	return nil, nil
}
