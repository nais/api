package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/api/internal/kubernetes/watcher"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	cnpgClusterGVR = schema.GroupVersionResource{
		Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters",
	}
	barmanObjectStoreGVR = schema.GroupVersionResource{
		Group: "barmancloud.cnpg.io", Version: "v1", Resource: "objectstores",
	}
)

func EarliestRestoreTime(ctx context.Context, branch *PostgresBranch) (*time.Time, error) {
	if branch.ClusterName == "" {
		return nil, nil
	}
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(
		ctx, branch.EnvironmentName, watcher.WithImpersonatedClientGVR(cnpgClusterGVR),
	)
	if err != nil {
		return nil, err
	}
	clusterName := branch.clusterName()
	cluster, err := client.Namespace(branch.TeamSlug.String()).Get(ctx, clusterName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG cluster %q for Postgres branch %q: %w", clusterName, branch.Name, err)
	}

	objectStoreName, err := barmanObjectStoreName(cluster)
	if err != nil {
		return nil, fmt.Errorf("reading Barman ObjectStore name from CNPG cluster %q: %w", clusterName, err)
	}
	if objectStoreName != "" {
		objectStoreClient, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(
			ctx, branch.EnvironmentName, watcher.WithImpersonatedClientGVR(barmanObjectStoreGVR),
		)
		if err != nil {
			return nil, err
		}
		objectStore, err := objectStoreClient.Namespace(branch.TeamSlug.String()).Get(ctx, objectStoreName, metav1.GetOptions{})
		if err != nil && !k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("getting Barman ObjectStore %q for CNPG cluster %q: %w", objectStoreName, clusterName, err)
		}
		if err == nil {
			earliest, err := firstRecoverabilityPoint(objectStore, clusterName)
			if err != nil || earliest != nil {
				return earliest, err
			}
		}
	}

	// Older CNPG configurations publish the recovery point directly on Cluster.
	value, found, err := unstructured.NestedString(cluster.Object, "status", "firstRecoverabilityPoint")
	if err != nil {
		return nil, fmt.Errorf("reading firstRecoverabilityPoint from CNPG cluster %q: %w", clusterName, err)
	}
	if !found || value == "" {
		return nil, nil
	}
	earliest, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parsing firstRecoverabilityPoint %q from CNPG cluster %q: %w", value, clusterName, err)
	}
	return &earliest, nil
}

func barmanObjectStoreName(cluster *unstructured.Unstructured) (string, error) {
	plugins, found, err := unstructured.NestedSlice(cluster.Object, "spec", "plugins")
	if err != nil || !found {
		return "", err
	}
	for _, item := range plugins {
		plugin, ok := item.(map[string]any)
		if !ok {
			return "", fmt.Errorf("CNPG plugin configuration has unexpected type %T", item)
		}
		name, found, err := unstructured.NestedString(plugin, "parameters", "barmanObjectName")
		if err != nil {
			return "", err
		}
		if found && name != "" {
			return name, nil
		}
	}
	return "", nil
}

func firstRecoverabilityPoint(objectStore *unstructured.Unstructured, clusterName string) (*time.Time, error) {
	value, found, err := unstructured.NestedString(objectStore.Object, "status", "serverRecoveryWindow", clusterName, "firstRecoverabilityPoint")
	if err != nil {
		return nil, err
	}
	if !found || value == "" {
		return nil, nil
	}
	earliest, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, fmt.Errorf("parsing firstRecoverabilityPoint %q: %w", value, err)
	}
	return &earliest, nil
}
