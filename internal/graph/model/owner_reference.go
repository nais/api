package model

import (
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func OwnerReference(refs []v1.OwnerReference) *v1.OwnerReference {
	if len(refs) == 0 {
		return nil
	}

	for _, o := range refs {
		if o.Kind == "Naisjob" || o.Kind == "Application" {
			return &v1.OwnerReference{
				APIVersion: o.APIVersion,
				Kind:       o.Kind,
				Name:       o.Name,
				UID:        o.UID,
			}
		}
	}
	return nil
}
