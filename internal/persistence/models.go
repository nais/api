package persistence

import (
	"github.com/nais/api/internal/graphv1/modelv1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Persistence interface {
	modelv1.Node
	IsPersistence()
}

func OwnerReference(refs []metav1.OwnerReference) *metav1.OwnerReference {
	if len(refs) == 0 {
		return nil
	}

	for _, o := range refs {
		if o.Kind == "Naisjob" || o.Kind == "Application" {
			return &metav1.OwnerReference{
				APIVersion: o.APIVersion,
				Kind:       o.Kind,
				Name:       o.Name,
				UID:        o.UID,
			}
		}
	}
	return nil
}
