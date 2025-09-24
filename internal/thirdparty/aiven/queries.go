package aiven

import (
	"context"
	"fmt"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/environmentmapper"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func GetProject(ctx context.Context, environmentName string) (Project, error) {
	clusterName := environmentmapper.ClusterName(environmentName)
	project, ok := fromContext(ctx).projects[clusterName]
	if !ok {
		return Project{}, fmt.Errorf("aiven project not found for cluster: %s", clusterName)
	}
	return project, nil
}

type Impersonator interface {
	ImpersonatedClient(ctx context.Context, environmentName string, opts ...watcher.ImpersonatedClientOption) (dynamic.NamespaceableResourceInterface, error)
}

func UpsertPrometheusServiceIntegration(ctx context.Context, impersonator Impersonator, owner *unstructured.Unstructured, project Project, environmentName string) error {
	name := owner.GetName()
	namespace := owner.GetNamespace()

	client, err := impersonator.ImpersonatedClient(ctx, environmentName, watcher.WithImpersonatedClientGVR(
		schema.GroupVersionResource{
			Group:    "aiven.io",
			Version:  "v1alpha1",
			Resource: "serviceintegrations",
		}))
	if err != nil {
		return fmt.Errorf("creating impersonated client: %w", err)
	}

	res := &unstructured.Unstructured{}
	existing, err := client.Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("getting existing ServiceIntegration: %w", err)
	} else if err == nil {
		res = existing
	}

	res.SetAPIVersion("aiven.io/v1alpha1")
	res.SetKind("ServiceIntegration")
	res.SetName(name)
	res.SetNamespace(namespace)
	res.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(res)

	res.Object["spec"] = map[string]any{
		"project":               project.ID,
		"integrationType":       "prometheus",
		"sourceServiceName":     name,
		"destinationEndpointId": project.EndpointID,
	}

	res.SetOwnerReferences([]metav1.OwnerReference{
		{
			APIVersion: owner.GetAPIVersion(),
			Kind:       owner.GetKind(),
			Name:       owner.GetName(),
			UID:        owner.GetUID(),
		},
	})

	if existing == nil {
		_, err = client.Namespace(namespace).Create(ctx, res, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("creating ServiceIntegration: %w", err)
		}
	} else {
		_, err := client.Namespace(namespace).Update(ctx, res, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("updating ServiceIntegration: %w", err)
		}
	}
	return nil
}
