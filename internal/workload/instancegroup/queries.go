package instancegroup

import (
	"context"
	"fmt"
	"maps"
	"path"
	"slices"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/dynamic"
)

// ListForApplication returns all instance groups for an application, sorted by revision (newest first).
// Only groups with replicas > 0 (desired or actual) are included.
func ListForApplication(ctx context.Context, teamSlug slug.Slug, environmentName, appName string) ([]*InstanceGroup, error) {
	l := fromContext(ctx)

	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{appName})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*nameReq)

	replicaSets := l.rsWatcher.GetByNamespace(
		teamSlug.String(),
		watcher.WithLabels(selector),
		watcher.InCluster(environmentName),
	)

	ret := make([]*InstanceGroup, 0, len(replicaSets))
	for _, rs := range replicaSets {
		ig := toGraphInstanceGroup(rs.Obj, rs.Cluster)
		// Only include groups that have running or desired instances
		if ig.DesiredInstances > 0 || ig.ReadyInstances > 0 {
			ret = append(ret, ig)
		}
	}

	// Sort by creation time, newest first
	slices.SortFunc(ret, func(a, b *InstanceGroup) int {
		return b.Created.Compare(a.Created)
	})

	return ret, nil
}

// GetByIdent returns an instance group by its ident.
func GetByIdent(ctx context.Context, id ident.Ident) (*InstanceGroup, error) {
	teamSlug, env, _, igName, err := parseIdent(id)
	if err != nil {
		return nil, err
	}

	l := fromContext(ctx)
	rs, err := l.rsWatcher.Get(env, teamSlug.String(), igName)
	if err != nil {
		return nil, err
	}

	return toGraphInstanceGroup(rs, env), nil
}

// ListEnvironmentVariables extracts environment variables from the instance group's pod template.
// For envFrom references, it resolves individual key names by querying the Kubernetes API.
// Variables from Secrets have nil values (require elevation). Variables from ConfigMaps include values.
func ListEnvironmentVariables(ctx context.Context, ig *InstanceGroup) ([]*InstanceGroupEnvironmentVariable, error) {
	if len(ig.PodTemplateSpec.Spec.Containers) == 0 {
		return nil, nil
	}

	l := fromContext(ctx)
	container := ig.PodTemplateSpec.Spec.Containers[0]
	var envVars []*InstanceGroupEnvironmentVariable

	// Direct env vars
	for _, env := range container.Env {
		ev := &InstanceGroupEnvironmentVariable{
			Name: env.Name,
		}

		switch {
		case env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil:
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSecret,
				Name: env.ValueFrom.SecretKeyRef.LocalObjectReference.Name,
			}
		case env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil:
			ref := env.ValueFrom.ConfigMapKeyRef
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindConfig,
				Name: ref.LocalObjectReference.Name,
			}
			data, err := getConfigMapData(ctx, l, ig.EnvironmentName, ig.TeamSlug.String(), ref.LocalObjectReference.Name)
			if err != nil {
				if ref.Optional != nil && *ref.Optional {
					l.log.WithError(err).WithField("configmap", ref.LocalObjectReference.Name).Debug("optional configmap key ref not found")
				} else {
					l.log.WithError(err).WithField("configmap", ref.LocalObjectReference.Name).Warn("failed to resolve configmap key ref")
				}
			} else if val, ok := data[ref.Key]; ok {
				ev.Value = &val
			}
		case env.ValueFrom != nil && env.ValueFrom.FieldRef != nil:
			value := "(" + env.ValueFrom.FieldRef.FieldPath + ")"
			ev.Value = &value
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSpec,
				Name: "fieldRef",
			}
		case env.ValueFrom != nil && env.ValueFrom.ResourceFieldRef != nil:
			value := "(" + env.ValueFrom.ResourceFieldRef.Resource + ")"
			ev.Value = &value
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSpec,
				Name: "resourceFieldRef",
			}
		default:
			value := env.Value
			ev.Value = &value
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSpec,
				Name: ig.ApplicationName,
			}
		}

		envVars = append(envVars, ev)
	}

	// envFrom sources - resolve individual key names from the referenced Secret/ConfigMap
	for _, envFrom := range container.EnvFrom {
		prefix := envFrom.Prefix

		switch {
		case envFrom.SecretRef != nil:
			secretName := envFrom.SecretRef.Name
			keys, err := getSecretKeys(ctx, l, ig.EnvironmentName, ig.TeamSlug.String(), secretName)
			if err != nil {
				l.log.WithError(err).WithField("secret", secretName).Warn("failed to resolve secret keys for envFrom")
				envVars = append(envVars, &InstanceGroupEnvironmentVariable{
					Name: fmt.Sprintf("(unable to resolve keys from Secret %s)", secretName),
					Source: InstanceGroupValueSource{
						Kind: InstanceGroupValueSourceKindSecret,
						Name: secretName,
					},
				})
				continue
			}
			for _, key := range keys {
				envVars = append(envVars, &InstanceGroupEnvironmentVariable{
					Name: prefix + key,
					Source: InstanceGroupValueSource{
						Kind: InstanceGroupValueSourceKindSecret,
						Name: secretName,
					},
				})
			}

		case envFrom.ConfigMapRef != nil:
			cmName := envFrom.ConfigMapRef.Name
			data, err := getConfigMapData(ctx, l, ig.EnvironmentName, ig.TeamSlug.String(), cmName)
			if err != nil {
				l.log.WithError(err).WithField("configmap", cmName).Warn("failed to resolve configmap data for envFrom")
				envVars = append(envVars, &InstanceGroupEnvironmentVariable{
					Name: fmt.Sprintf("(unable to resolve keys from ConfigMap %s)", cmName),
					Source: InstanceGroupValueSource{
						Kind: InstanceGroupValueSourceKindConfig,
						Name: cmName,
					},
				})
				continue
			}
			for _, key := range slices.Sorted(maps.Keys(data)) {
				value := data[key]
				envVars = append(envVars, &InstanceGroupEnvironmentVariable{
					Name:  prefix + key,
					Value: &value,
					Source: InstanceGroupValueSource{
						Kind: InstanceGroupValueSourceKindConfig,
						Name: cmName,
					},
				})
			}
		}
	}

	return envVars, nil
}

// ListMountedFiles extracts mounted files (from Secrets/ConfigMaps) from the instance group's pod template.
// Files are expanded to individual paths by resolving key names from the Kubernetes API.
func ListMountedFiles(ctx context.Context, ig *InstanceGroup) ([]*InstanceGroupMountedFile, error) {
	if len(ig.PodTemplateSpec.Spec.Containers) == 0 {
		return nil, nil
	}

	l := fromContext(ctx)

	// Build a map of volume name -> volume source for lookup
	volumeMap := make(map[string]corev1.Volume)
	for _, vol := range ig.PodTemplateSpec.Spec.Volumes {
		volumeMap[vol.Name] = vol
	}

	container := ig.PodTemplateSpec.Spec.Containers[0]
	var files []*InstanceGroupMountedFile

	for _, mount := range container.VolumeMounts {
		vol, ok := volumeMap[mount.Name]
		if !ok {
			continue
		}

		switch {
		case vol.Secret != nil:
			expanded := expandSecretVolume(ctx, l, ig, mount, vol.Secret.SecretName, vol.Secret.Items)
			files = append(files, expanded...)

		case vol.ConfigMap != nil:
			expanded := expandConfigMapVolume(ctx, l, ig, mount, vol.ConfigMap.Name, vol.ConfigMap.Items)
			files = append(files, expanded...)

		case vol.Projected != nil:
			expanded := expandProjectedVolume(ctx, l, ig, mount, vol.Projected)
			files = append(files, expanded...)

		default:
			// Skip volumes that aren't from secrets/configmaps (emptyDir, hostPath, etc.)
			continue
		}
	}

	return files, nil
}

// expandSecretVolume expands a secret volume mount into individual file entries.
// Secret file content is not included (requires elevation to view).
func expandSecretVolume(ctx context.Context, l *loaders, ig *InstanceGroup, mount corev1.VolumeMount, secretName string, items []corev1.KeyToPath) []*InstanceGroupMountedFile {
	source := InstanceGroupValueSource{
		Kind: InstanceGroupValueSourceKindSecret,
		Name: secretName,
	}

	// subPath means a single key is mounted directly at mountPath
	if mount.SubPath != "" {
		return []*InstanceGroupMountedFile{{
			Path:   mount.MountPath,
			Source: source,
		}}
	}

	// If items are specified, only those keys are mounted
	if len(items) > 0 {
		files := make([]*InstanceGroupMountedFile, 0, len(items))
		for _, item := range items {
			files = append(files, &InstanceGroupMountedFile{
				Path:   path.Join(mount.MountPath, item.Path),
				Source: source,
			})
		}
		return files
	}

	// No items specified - all keys are mounted as files. Resolve from K8s API.
	keys, err := getSecretKeys(ctx, l, ig.EnvironmentName, ig.TeamSlug.String(), secretName)
	if err != nil {
		l.log.WithError(err).WithField("secret", secretName).Warn("failed to resolve secret keys for volume mount")
		errMsg := fmt.Sprintf("Secret '%s' could not be found or accessed. The application may fail to start until this is resolved.", secretName)
		return []*InstanceGroupMountedFile{{
			Path:   mount.MountPath,
			Source: source,
			Error:  &errMsg,
		}}
	}

	files := make([]*InstanceGroupMountedFile, 0, len(keys))
	for _, key := range keys {
		files = append(files, &InstanceGroupMountedFile{
			Path:   path.Join(mount.MountPath, key),
			Source: source,
		})
	}
	return files
}

// expandConfigMapVolume expands a configmap volume mount into individual file entries with content.
func expandConfigMapVolume(ctx context.Context, l *loaders, ig *InstanceGroup, mount corev1.VolumeMount, cmName string, items []corev1.KeyToPath) []*InstanceGroupMountedFile {
	source := InstanceGroupValueSource{
		Kind: InstanceGroupValueSourceKindConfig,
		Name: cmName,
	}

	// subPath means a single key is mounted directly at mountPath
	if mount.SubPath != "" {
		// Fetch content for the specific key
		contents, err := getConfigMapFileContents(ctx, l, ig.EnvironmentName, ig.TeamSlug.String(), cmName)
		if err != nil {
			l.log.WithError(err).WithField("configmap", cmName).Warn("failed to resolve configmap content for subPath mount")
			errMsg := fmt.Sprintf("ConfigMap '%s' could not be found or accessed. The application may fail to start until this is resolved.", cmName)
			return []*InstanceGroupMountedFile{{
				Path:   mount.MountPath,
				Source: source,
				Error:  &errMsg,
			}}
		}
		// When items are specified, SubPath matches item.Path, not item.Key.
		// Map SubPath back to the actual ConfigMap key via the items list.
		key := mount.SubPath
		for _, item := range items {
			if item.Path == mount.SubPath {
				key = item.Key
				break
			}
		}
		f := &InstanceGroupMountedFile{
			Path:   mount.MountPath,
			Source: source,
		}
		if fc, ok := contents[key]; ok {
			f.Content = &fc.content
			f.IsBinary = fc.isBinary
		}
		return []*InstanceGroupMountedFile{f}
	}

	// Fetch all file contents from the ConfigMap
	contents, err := getConfigMapFileContents(ctx, l, ig.EnvironmentName, ig.TeamSlug.String(), cmName)
	if err != nil {
		l.log.WithError(err).WithField("configmap", cmName).Warn("failed to resolve configmap content for volume mount")
		errMsg := fmt.Sprintf("ConfigMap '%s' could not be found or accessed. The application may fail to start until this is resolved.", cmName)
		return []*InstanceGroupMountedFile{{
			Path:   mount.MountPath,
			Source: source,
			Error:  &errMsg,
		}}
	}

	// If items are specified, only those keys are mounted
	if len(items) > 0 {
		files := make([]*InstanceGroupMountedFile, 0, len(items))
		for _, item := range items {
			f := &InstanceGroupMountedFile{
				Path:   path.Join(mount.MountPath, item.Path),
				Source: source,
			}
			if fc, ok := contents[item.Key]; ok {
				f.Content = &fc.content
				f.IsBinary = fc.isBinary
			}
			files = append(files, f)
		}
		return files
	}

	// No items specified - all keys are mounted as files.
	keys := slices.Sorted(maps.Keys(contents))
	files := make([]*InstanceGroupMountedFile, 0, len(keys))
	for _, key := range keys {
		fc := contents[key]
		content := fc.content
		files = append(files, &InstanceGroupMountedFile{
			Path:     path.Join(mount.MountPath, key),
			Source:   source,
			Content:  &content,
			IsBinary: fc.isBinary,
		})
	}
	return files
}

// expandProjectedVolume expands a projected volume mount into individual file entries.
func expandProjectedVolume(ctx context.Context, l *loaders, ig *InstanceGroup, mount corev1.VolumeMount, projected *corev1.ProjectedVolumeSource) []*InstanceGroupMountedFile {
	// subPath means a single file is mounted directly at mountPath
	if mount.SubPath != "" {
		// For projected volumes with subPath, use the first source as representative
		source := InstanceGroupValueSource{Kind: InstanceGroupValueSourceKindSpec, Name: "projected"}
		for _, src := range projected.Sources {
			if src.Secret != nil {
				source = InstanceGroupValueSource{Kind: InstanceGroupValueSourceKindSecret, Name: src.Secret.Name}
				break
			}
			if src.ConfigMap != nil {
				source = InstanceGroupValueSource{Kind: InstanceGroupValueSourceKindConfig, Name: src.ConfigMap.Name}
				break
			}
		}
		return []*InstanceGroupMountedFile{{
			Path:   mount.MountPath,
			Source: source,
		}}
	}

	var files []*InstanceGroupMountedFile

	for _, src := range projected.Sources {
		switch {
		case src.Secret != nil:
			expanded := expandSecretVolume(ctx, l, ig, corev1.VolumeMount{
				MountPath: mount.MountPath,
			}, src.Secret.Name, src.Secret.Items)
			files = append(files, expanded...)

		case src.ConfigMap != nil:
			expanded := expandConfigMapVolume(ctx, l, ig, corev1.VolumeMount{
				MountPath: mount.MountPath,
			}, src.ConfigMap.Name, src.ConfigMap.Items)
			files = append(files, expanded...)
		}
	}

	return files
}

// ListInstances returns the application instances belonging to this instance group.
// It matches pods by their ownerReference to the ReplicaSet.
func ListInstances(ctx context.Context, ig *InstanceGroup) ([]*application.ApplicationInstance, error) {
	l := fromContext(ctx)

	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{ig.ApplicationName})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*nameReq)

	pods := l.podWatcher.GetByNamespace(
		ig.TeamSlug.String(),
		watcher.WithLabels(selector),
		watcher.InCluster(ig.EnvironmentName),
	)

	var instances []*application.ApplicationInstance
	for _, pod := range pods {
		// Match pod to this instance group via ownerReferences
		for _, ref := range pod.Obj.OwnerReferences {
			if ref.Kind == "ReplicaSet" && ref.Name == ig.Name {
				instances = append(instances, toApplicationInstance(pod.Obj, ig.TeamSlug, ig.EnvironmentName, ig.ApplicationName))
				break
			}
		}
	}

	return instances, nil
}

// toApplicationInstance converts a pod to an ApplicationInstance.
// This mirrors the logic in the application package.
func toApplicationInstance(pod *corev1.Pod, teamSlug slug.Slug, environmentName, applicationName string) *application.ApplicationInstance {
	var containerStatus corev1.ContainerStatus
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == applicationName {
			containerStatus = c
			break
		}
	}

	var imageString string
	if len(pod.Spec.Containers) > 0 {
		imageString = pod.Spec.Containers[0].Image
	}

	return &application.ApplicationInstance{
		Name:                       pod.Name,
		Restarts:                   int(containerStatus.RestartCount),
		Created:                    pod.CreationTimestamp.Time,
		EnvironmentName:            environmentName,
		ImageString:                imageString,
		TeamSlug:                   teamSlug,
		ApplicationName:            applicationName,
		ApplicationContainerStatus: containerStatus,
	}
}

// getSecretKeys fetches the key names from a Secret in the given namespace via direct K8s API call.
// Only key names are returned, not values.
func getSecretKeys(ctx context.Context, l *loaders, environmentName, namespace, secretName string) ([]string, error) {
	client, err := l.k8sClient(environmentName)
	if err != nil {
		return nil, err
	}

	obj, err := secretResource(client).Namespace(namespace).Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get secret %s/%s: %w", namespace, secretName, err)
	}

	data, _, _ := unstructured.NestedMap(obj.Object, "data")
	return slices.Sorted(maps.Keys(data)), nil
}

// getConfigMapData fetches the key-value data from a ConfigMap in the given namespace via direct K8s API call.
func getConfigMapData(ctx context.Context, l *loaders, environmentName, namespace, cmName string) (map[string]string, error) {
	client, err := l.k8sClient(environmentName)
	if err != nil {
		return nil, err
	}

	obj, err := configMapResource(client).Namespace(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s: %w", namespace, cmName, err)
	}

	data, _, _ := unstructured.NestedStringMap(obj.Object, "data")
	return data, nil
}

// configMapFileContent holds the content of a ConfigMap key with binary metadata.
type configMapFileContent struct {
	content  string
	isBinary bool
}

// getConfigMapFileContents fetches all file contents from a ConfigMap, including binaryData entries.
// String data values are returned as-is. Binary data values are returned as base64-encoded strings with isBinary=true.
func getConfigMapFileContents(ctx context.Context, l *loaders, environmentName, namespace, cmName string) (map[string]configMapFileContent, error) {
	client, err := l.k8sClient(environmentName)
	if err != nil {
		return nil, err
	}

	obj, err := configMapResource(client).Namespace(namespace).Get(ctx, cmName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get configmap %s/%s: %w", namespace, cmName, err)
	}

	result := make(map[string]configMapFileContent)

	// String data
	data, _, _ := unstructured.NestedStringMap(obj.Object, "data")
	for k, v := range data {
		result[k] = configMapFileContent{content: v, isBinary: false}
	}

	// Binary data (base64-encoded in the K8s API response)
	binaryData, _, _ := unstructured.NestedMap(obj.Object, "binaryData")
	for k, v := range binaryData {
		if s, ok := v.(string); ok {
			result[k] = configMapFileContent{content: s, isBinary: true}
		}
	}

	return result, nil
}

func secretResource(client dynamic.Interface) dynamic.NamespaceableResourceInterface {
	return client.Resource(corev1.SchemeGroupVersion.WithResource("secrets"))
}

func configMapResource(client dynamic.Interface) dynamic.NamespaceableResourceInterface {
	return client.Resource(corev1.SchemeGroupVersion.WithResource("configmaps"))
}
