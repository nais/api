package postgres

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"time"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/kubernetes"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

var (
	cnpgClusterGVR = schema.GroupVersionResource{
		Group: "postgresql.cnpg.io", Version: "v1", Resource: "clusters",
	}
	barmanObjectStoreGVR = schema.GroupVersionResource{
		Group: "barmancloud.cnpg.io", Version: "v1", Resource: "objectstores",
	}
	postgresRecoveryGVR = schema.GroupVersionResource{
		Group: "nais.io", Version: "v1", Resource: "postgresrecoveries",
	}
)

func postgresRecoveryClient(ctx context.Context, teamSlug slug.Slug, environmentName string) (dynamic.ResourceInterface, error) {
	c, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(
		ctx, environmentName, watcher.WithImpersonatedClientGVR(postgresRecoveryGVR),
	)
	if err != nil {
		return nil, err
	}
	return c.Namespace(teamSlug.String()), nil
}

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
// from clients. Resolve and pin the live source's UID and physical archive in a
// companion PostgresRecovery resource.
func resolveRecovery(ctx context.Context, client dynamic.ResourceInterface, input CreatePostgresInput, target *nais_io_v1.Postgres) (*nais_io_v1.PostgresRecovery, error) {
	if input.Recovery != nil {
		return resolvePointInTimeRecovery(ctx, client, input, target)
	}
	if input.BackupName != nil {
		return resolveBackupRecovery(ctx, input, target)
	}
	return nil, nil
}

func resolvePointInTimeRecovery(ctx context.Context, client dynamic.ResourceInterface, input CreatePostgresInput, target *nais_io_v1.Postgres) (*nais_io_v1.PostgresRecovery, error) {
	sourceObject, err := client.Get(ctx, input.Recovery.SourcePostgres, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Source Postgres %q not found in this team and environment.", input.Recovery.SourcePostgres)
	}
	if err != nil {
		return nil, err
	}
	source, err := kubernetes.ToConcrete[nais_io_v1.Postgres](sourceObject)
	if err != nil {
		return nil, err
	}
	if source.UID == "" || !source.DeletionTimestamp.IsZero() || source.Status == nil || source.Status.ClusterName == "" {
		return nil, apierror.Errorf("Source Postgres is deleting or has no recoverable cluster yet.")
	}
	if source.Spec.MajorVersion != input.MajorVersion {
		return nil, apierror.Errorf("Recovery must use the source PostgreSQL major version %s.", source.Spec.MajorVersion)
	}
	if input.Extensions == nil {
		target.Spec.Extensions = slices.Clone(source.Spec.Extensions)
	}
	for _, extension := range source.Spec.Extensions {
		if !slices.Contains(target.Spec.Extensions, extension) {
			return nil, apierror.Errorf("Recovery must include source extension %q.", extension.Name)
		}
	}
	// Unspecified resources inherit the source configuration, particularly disk
	// size: the normal empty-database defaults can be too small for a restore.
	target.Spec.Resources = *source.Spec.Resources.DeepCopy()
	if input.DiskSize != nil && source.Spec.Resources.DiskSize.Sign() > 0 {
		requested, err := resource.ParseQuantity(*input.DiskSize)
		if err != nil {
			return nil, err
		}
		if requested.Cmp(source.Spec.Resources.DiskSize) < 0 {
			return nil, apierror.Errorf("Recovery disk size must not be smaller than the source (%s).", source.Spec.Resources.DiskSize.String())
		}
	}
	if input.HighAvailability == nil {
		target.Spec.HighAvailability = source.Spec.HighAvailability
	}
	pg, err := toPostgres(sourceObject, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	earliest, err := PostgresEarliestRestoreTime(ctx, pg)
	if err != nil {
		return nil, err
	}
	if earliest == nil {
		return nil, apierror.Errorf("Source Postgres has no recoverable backup window yet.")
	}
	if input.Recovery.TargetTime.Before(*earliest) {
		return nil, apierror.Errorf("Restore time must not be before the earliest recoverable time %s.", earliest.UTC().Format(time.RFC3339))
	}
	clusters, err := fromContext(ctx).postgresWatcher.SystemAuthenticatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(cnpgClusterGVR))
	if err != nil {
		return nil, err
	}
	cluster, err := clusters.Namespace(input.TeamSlug.String()).Get(ctx, source.Status.ClusterName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if !metav1.IsControlledBy(cluster, source) || !cluster.GetDeletionTimestamp().IsZero() {
		return nil, apierror.Errorf("Source cluster does not belong to the selected Postgres or is deleting.")
	}
	if source.Status.Ownership == nil || source.Status.Ownership.Phase != "Managed" || source.Status.Ownership.ClusterUID == "" || source.Status.Ownership.ClusterUID != string(cluster.GetUID()) {
		return nil, apierror.Errorf("Source Postgres has no matching managed cluster checkpoint.")
	}
	bucket, err := barmanObjectStoreName(cluster)
	if err != nil {
		return nil, err
	}
	if bucket == "" {
		return nil, apierror.Errorf("Source Postgres has no Barman archive.")
	}
	markTargetForCompanionRecovery(target)
	return &nais_io_v1.PostgresRecovery{
		TypeMeta:   metav1.TypeMeta{Kind: "PostgresRecovery", APIVersion: nais_io_v1.GroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: target.Name, Namespace: target.Namespace},
		Spec: nais_io_v1.PostgresRecoverySpec{
			Postgres: target.Name,
			PointInTime: &nais_io_v1.PostgresPointInTimeRecovery{
				SourcePostgres: source.Name,
				SourceUID:      string(source.UID),
				BucketName:     bucket,
				ServerName:     source.Status.ClusterName,
				TargetTime:     metav1.NewTime(input.Recovery.TargetTime.UTC()),
			},
		},
	}, nil
}

func resolveBackupRecovery(ctx context.Context, input CreatePostgresInput, target *nais_io_v1.Postgres) (*nais_io_v1.PostgresRecovery, error) {
	if err := authz.CanReadPostgresBackups(ctx, input.TeamSlug); err != nil {
		return nil, err
	}
	backupClient, err := postgresBackupClient(ctx, input.TeamSlug, input.EnvironmentName)
	if err != nil {
		return nil, err
	}
	backupObject, err := backupClient.Get(ctx, *input.BackupName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, apierror.Errorf("Standalone backup %q not found in this team and environment.", *input.BackupName)
	}
	if err != nil {
		return nil, err
	}
	backup, err := kubernetes.ToConcrete[nais_io_v1.PostgresBackup](backupObject)
	if err != nil {
		return nil, err
	}
	if backup.UID == "" || !backup.DeletionTimestamp.IsZero() || backup.Status == nil || backup.Status.Phase != nais_io_v1.PostgresBackupPhaseCompleted {
		return nil, apierror.Errorf("Standalone backup %q is not completed or is being deleted.", *input.BackupName)
	}
	if backup.Status.ExpiresAt != nil && !time.Now().Before(backup.Status.ExpiresAt.Time) {
		return nil, apierror.Errorf("Standalone backup %q has expired.", *input.BackupName)
	}
	if backup.Status.Location == nil || backup.Status.Location.BucketName == "" || backup.Status.Location.Prefix == "" ||
		backup.Status.ServerName == "" || backup.Status.BackupID == "" {
		return nil, apierror.Errorf("Standalone backup %q has incomplete archive metadata.", *input.BackupName)
	}
	if backup.Status.MajorVersion != "" && backup.Status.MajorVersion != input.MajorVersion {
		return nil, apierror.Errorf("Recovery must use the backup PostgreSQL major version %s.", backup.Status.MajorVersion)
	}
	if input.Extensions == nil {
		target.Spec.Extensions = slices.Clone(backup.Status.Extensions)
	}
	for _, extension := range backup.Status.Extensions {
		if !slices.Contains(target.Spec.Extensions, extension) {
			return nil, apierror.Errorf("Recovery must include backup extension %q.", extension.Name)
		}
	}
	if backup.Status.DiskSize != nil && backup.Status.DiskSize.Sign() > 0 {
		target.Spec.Resources.DiskSize = backup.Status.DiskSize.DeepCopy()
		if input.DiskSize != nil {
			requested, err := resource.ParseQuantity(*input.DiskSize)
			if err != nil {
				return nil, err
			}
			if requested.Cmp(*backup.Status.DiskSize) < 0 {
				return nil, apierror.Errorf("Recovery disk size must not be smaller than the backup (%s).", backup.Status.DiskSize.String())
			}
		}
	}
	markTargetForCompanionRecovery(target)
	return &nais_io_v1.PostgresRecovery{
		TypeMeta:   metav1.TypeMeta{Kind: "PostgresRecovery", APIVersion: nais_io_v1.GroupVersion.String()},
		ObjectMeta: metav1.ObjectMeta{Name: target.Name, Namespace: target.Namespace},
		Spec: nais_io_v1.PostgresRecoverySpec{
			Postgres: target.Name,
			Backup: &nais_io_v1.PostgresBackupRecovery{
				SourcePostgres: backup.Spec.Postgres,
				BackupName:     backup.Name,
				BackupUID:      string(backup.UID),
				BucketName:     backup.Status.Location.BucketName,
				Prefix:         backup.Status.Location.Prefix,
				ServerName:     backup.Status.ServerName,
				BackupID:       backup.Status.BackupID,
			},
		},
	}, nil
}

func markTargetForCompanionRecovery(target *nais_io_v1.Postgres) {
	annotations := target.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[nais_io_v1.PostgresRecoveryAnnotation] = target.Name
	target.SetAnnotations(annotations)
}

func postgresRecoveryFromResource(rec *nais_io_v1.PostgresRecovery, completed bool, failureMessage *string) *PostgresRecovery {
	if rec == nil {
		return nil
	}
	if rec.Spec.PointInTime != nil {
		targetTime := rec.Spec.PointInTime.TargetTime.Time
		return &PostgresRecovery{
			SourcePostgres: rec.Spec.PointInTime.SourcePostgres,
			TargetTime:     &targetTime,
			Completed:      completed,
			FailureMessage: failureMessage,
		}
	}
	if rec.Spec.Backup != nil {
		backupName := rec.Spec.Backup.BackupName
		return &PostgresRecovery{
			SourcePostgres: rec.Spec.Backup.SourcePostgres,
			BackupName:     &backupName,
			Completed:      completed,
			FailureMessage: failureMessage,
		}
	}
	return nil
}

// RecoveryForPostgres resolves the recovery metadata for a Postgres resource
// without mutating shared watcher objects.
func RecoveryForPostgres(ctx context.Context, pg *Postgres) (*PostgresRecovery, error) {
	if pg == nil {
		return nil, nil
	}
	if pg.inlineRecovery != nil {
		return pg.inlineRecovery, nil
	}
	if pg.recoveryResourceName == "" {
		return nil, nil
	}
	recClient, err := postgresRecoveryClient(ctx, pg.TeamSlug, pg.EnvironmentName)
	if err != nil {
		return nil, nil
	}
	recObj, err := recClient.Get(ctx, pg.recoveryResourceName, metav1.GetOptions{})
	if k8serrors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rec, err := kubernetes.ToConcrete[nais_io_v1.PostgresRecovery](recObj)
	if err != nil {
		return nil, err
	}
	return postgresRecoveryFromResource(rec, pg.recoveryCompleted, pg.recoveryFailure), nil
}

func createCompanionRecovery(ctx context.Context, recClient dynamic.ResourceInterface, companionRecovery *nais_io_v1.PostgresRecovery, created *unstructured.Unstructured) error {
	companionRecovery.SetAnnotations(kubernetes.WithCommonAnnotations(nil, authz.ActorFromContext(ctx).User.Identity()))
	kubernetes.SetManagedByConsoleLabel(companionRecovery)
	if uid := created.GetUID(); uid != "" {
		companionRecovery.SetOwnerReferences([]metav1.OwnerReference{{
			APIVersion: nais_io_v1.GroupVersion.String(),
			Kind:       "Postgres",
			Name:       created.GetName(),
			UID:        uid,
		}})
	}
	recObj, err := kubernetes.ToUnstructured(companionRecovery)
	if err != nil {
		return err
	}
	if _, err := recClient.Create(ctx, recObj, metav1.CreateOptions{FieldValidation: metav1.FieldValidationStrict}); err != nil {
		if companionRecoveryPersistedForTarget(context.WithoutCancel(ctx), recClient, companionRecovery, created.GetUID()) {
			return nil
		}
		return err
	}
	return nil
}

func companionRecoveryPersistedForTarget(ctx context.Context, recClient dynamic.ResourceInterface, want *nais_io_v1.PostgresRecovery, targetUID types.UID) bool {
	recObj, err := recClient.Get(ctx, want.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	got, err := kubernetes.ToConcrete[nais_io_v1.PostgresRecovery](recObj)
	if err != nil || !reflect.DeepEqual(got.Spec, want.Spec) {
		return false
	}
	if targetUID == "" {
		return true
	}
	for _, owner := range got.OwnerReferences {
		if owner.Kind == "Postgres" && owner.Name == want.Spec.Postgres && owner.UID == targetUID {
			return true
		}
	}
	return false
}

func rollbackCreatedPostgres(ctx context.Context, client dynamic.ResourceInterface, created *unstructured.Unstructured) {
	cleanupCtx := context.WithoutCancel(ctx)
	deleteOpts := metav1.DeleteOptions{}
	if uid := created.GetUID(); uid != "" {
		deleteOpts.Preconditions = &metav1.Preconditions{UID: new(uid)}
	}
	_ = client.Delete(cleanupCtx, created.GetName(), deleteOpts)
}
