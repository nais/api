package kubernetes

import (
	"fmt"
	"time"

	"github.com/nais/api/internal/environmentmapper"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	labelManagedByKey           = "nais.io/managed-by"
	labelManagedByVal           = "console"
	labelKubernetesManagedByKey = "app.kubernetes.io/managed-by"

	annotationLastModifiedAt = "console.nais.io/last-modified-at"
	annotationLastModifiedBy = "console.nais.io/last-modified-by"
)

func NewClientSets(clusterConfig ClusterConfigMap) (map[string]kubernetes.Interface, error) {
	k8sClientSets := make(map[string]kubernetes.Interface)
	for cluster, cfg := range clusterConfig {
		if cfg == nil {
			var err error
			cfg, err = rest.InClusterConfig()
			if err != nil {
				return nil, fmt.Errorf("get in-cluster config for cluster %q: %w", cluster, err)
			}
		}
		clientSet, err := kubernetes.NewForConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("create k8s client set for cluster %q: %w", cluster, err)
		}
		k8sClientSets[environmentmapper.EnvironmentName(cluster)] = clientSet
	}

	return k8sClientSets, nil
}

func IsManagedByConsoleLabelSelector() string {
	return labelManagedByKey + "=" + labelManagedByVal
}

type LabeledObject interface {
	GetLabels() map[string]string
	SetLabels(map[string]string)
}

func HasManagedByConsoleLabel(obj LabeledObject) bool {
	lbls := obj.GetLabels()
	if lbls == nil {
		return false
	}
	managedBy, ok := lbls[labelManagedByKey]
	if !ok {
		return false
	}

	return managedBy == labelManagedByVal
}

func SetManagedByConsoleLabel(obj LabeledObject) {
	lbls := obj.GetLabels()
	if lbls == nil {
		lbls = make(map[string]string)
	}
	lbls[labelManagedByKey] = labelManagedByVal
	lbls[labelKubernetesManagedByKey] = labelManagedByVal
	obj.SetLabels(lbls)
}

func WithCommonAnnotations(mp map[string]string, user string) map[string]string {
	if mp == nil {
		mp = make(map[string]string)
	}
	mp[annotationLastModifiedAt] = time.Now().Format(time.RFC3339)
	if user != "" {
		mp[annotationLastModifiedBy] = user
	}

	return mp
}
