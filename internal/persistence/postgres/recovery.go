package postgres

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	cnpgClusterGVR = schema.GroupVersionResource{
		Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters",
	}
	barmanObjectStoreGVR = schema.GroupVersionResource{
		Group: "barmancloud.cnpg.io", Version: "v1", Resource: "objectstores",
	}
)

func PostgresEarliestRestoreTime(ctx context.Context, pg *Postgres) (*time.Time, error) {
	if pg.directClusterName == nil || *pg.directClusterName == "" {
		return nil, nil
	}
	client, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(
		ctx, pg.EnvironmentName, watcher.WithImpersonatedClientGVR(cnpgClusterGVR),
	)
	if err != nil {
		return nil, err
	}
	clusterName := *pg.directClusterName
	cluster, err := client.Namespace(pg.TeamSlug.String()).Get(ctx, clusterName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting CNPG cluster %q for Postgres %q: %w", clusterName, pg.Name, err)
	}

	objectStoreName, err := barmanObjectStoreName(cluster)
	if err != nil {
		return nil, fmt.Errorf("reading Barman ObjectStore name from CNPG cluster %q: %w", clusterName, err)
	}
	if objectStoreName != "" {
		objectStoreClient, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(
			ctx, pg.EnvironmentName, watcher.WithImpersonatedClientGVR(barmanObjectStoreGVR),
		)
		if err != nil {
			return nil, err
		}
		objectStore, err := objectStoreClient.Namespace(pg.TeamSlug.String()).Get(ctx, objectStoreName, metav1.GetOptions{})
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
		if plugin["name"] == "barman-cloud.cloudnative-pg.io" && found && name != "" {
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

// resolveRecovery never accepts bucket names or cross-team/environment selectors
// from clients. Resolve and pin the live source's UID and physical archive.
func resolveRecovery(ctx context.Context, client dynamic.ResourceInterface, input CreatePostgresInput, target *nais_io_v1.Postgres) error {
	if input.Recovery == nil {
		return nil
	}
	sourceObject, err := client.Get(ctx, input.Recovery.SourcePostgres, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return apierror.Errorf("Source Postgres %q not found in this team and environment.", input.Recovery.SourcePostgres)
	}
	if err != nil {
		return err
	}
	source, err := kubernetes.ToConcrete[nais_io_v1.Postgres](sourceObject)
	if err != nil {
		return err
	}
	if source.UID == "" || !source.DeletionTimestamp.IsZero() || source.Status == nil || source.Status.ClusterName == "" {
		return apierror.Errorf("Source Postgres is deleting or has no recoverable cluster yet.")
	}
	if source.Spec.MajorVersion != input.MajorVersion {
		return apierror.Errorf("Recovery must use the source PostgreSQL major version %s.", source.Spec.MajorVersion)
	}
	if input.Extensions == nil {
		target.Spec.Extensions = slices.Clone(source.Spec.Extensions)
	}
	for _, extension := range source.Spec.Extensions {
		if !slices.Contains(target.Spec.Extensions, extension) {
			return apierror.Errorf("Recovery must include source extension %q.", extension.Name)
		}
	}
	// Unspecified resources inherit the source configuration, particularly disk
	// size: the normal empty-database defaults can be too small for a restore.
	target.Spec.Resources = *source.Spec.Resources.DeepCopy()
	if input.DiskSize != nil && source.Spec.Resources.DiskSize.Sign() > 0 {
		requested, err := resource.ParseQuantity(*input.DiskSize)
		if err != nil {
			return err
		}
		if requested.Cmp(source.Spec.Resources.DiskSize) < 0 {
			return apierror.Errorf("Recovery disk size must not be smaller than the source (%s).", source.Spec.Resources.DiskSize.String())
		}
	}
	if input.HighAvailability == nil {
		target.Spec.HighAvailability = source.Spec.HighAvailability
	}
	pg, err := toPostgres(sourceObject, input.EnvironmentName)
	if err != nil {
		return err
	}
	earliest, err := PostgresEarliestRestoreTime(ctx, pg)
	if err != nil {
		return err
	}
	if earliest == nil {
		return apierror.Errorf("Source Postgres has no recoverable backup window yet.")
	}
	if input.Recovery.TargetTime.Before(*earliest) {
		return apierror.Errorf("Restore time must not be before the earliest recoverable time %s.", earliest.UTC().Format(time.RFC3339))
	}
	clusters, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(cnpgClusterGVR))
	if err != nil {
		return err
	}
	cluster, err := clusters.Namespace(input.TeamSlug.String()).Get(ctx, source.Status.ClusterName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if !metav1.IsControlledBy(cluster, source) || !cluster.GetDeletionTimestamp().IsZero() {
		return apierror.Errorf("Source cluster does not belong to the selected Postgres or is deleting.")
	}
	if source.Status.Ownership == nil || source.Status.Ownership.Phase != "Managed" || source.Status.Ownership.ClusterUID == "" || source.Status.Ownership.ClusterUID != string(cluster.GetUID()) {
		return apierror.Errorf("Source Postgres has no matching managed cluster checkpoint.")
	}
	bucket, err := barmanObjectStoreName(cluster)
	if err != nil {
		return err
	}
	if bucket == "" {
		return apierror.Errorf("Source Postgres has no Barman archive.")
	}
	target.Spec.Recovery = &nais_io_v1.PostgresRecovery{
		SourcePostgres: source.Name, SourceUID: string(source.UID), BucketName: bucket,
		ServerName: source.Status.ClusterName, TargetTime: metav1.NewTime(input.Recovery.TargetTime.UTC()),
	}
	return nil
}
