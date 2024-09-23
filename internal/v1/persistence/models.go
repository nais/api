package persistence

import (
	"github.com/nais/api/internal/v1/graphv1/modelv1"
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

func WorkloadReferenceFromOwnerReferences(ownerReferences []metav1.OwnerReference) *WorkloadReference {
	if len(ownerReferences) == 0 {
		return nil
	}

	for _, o := range ownerReferences {
		switch o.Kind {
		case "Naisjob":
			return &WorkloadReference{
				Name: o.Name,
				Type: WorkloadTypeJob,
			}
		case "Application":
			return &WorkloadReference{
				Name: o.Name,
				Type: WorkloadTypeApplication,
			}
		}
	}
	return nil
}

type WorkloadReference struct {
	Name string
	Type WorkloadType
}

type WorkloadType int

const (
	WorkloadTypeApplication WorkloadType = iota
	WorkloadTypeJob
)
