package instancegroup

import (
	"context"
	"sort"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload/application"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
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

	// Sort by revision, newest first
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Revision > ret[j].Revision
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
// Variables from Secrets are marked as requiring elevation; their values are not included.
// Variables from ConfigMaps or directly from spec have their values included.
func ListEnvironmentVariables(ctx context.Context, ig *InstanceGroup) []*InstanceGroupEnvironmentVariable {
	if len(ig.PodTemplateSpec.Spec.Containers) == 0 {
		return nil
	}

	container := ig.PodTemplateSpec.Spec.Containers[0]
	var envVars []*InstanceGroupEnvironmentVariable

	// Direct env vars
	for _, env := range container.Env {
		ev := &InstanceGroupEnvironmentVariable{
			Name: env.Name,
		}

		switch {
		case env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil:
			ev.RequiresElevation = true
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSecret,
				Name: env.ValueFrom.SecretKeyRef.LocalObjectReference.Name,
			}
		case env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil:
			value := env.Value // will be empty for valueFrom, but we try
			ev.Value = &value
			ev.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindConfigMap,
				Name: env.ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name,
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

	// envFrom sources (whole secret/configmap injected as env vars)
	for _, envFrom := range container.EnvFrom {
		switch {
		case envFrom.SecretRef != nil:
			envVars = append(envVars, &InstanceGroupEnvironmentVariable{
				Name:              "(all keys from " + envFrom.SecretRef.Name + ")",
				RequiresElevation: true,
				Source: InstanceGroupValueSource{
					Kind: InstanceGroupValueSourceKindSecret,
					Name: envFrom.SecretRef.Name,
				},
			})
		case envFrom.ConfigMapRef != nil:
			envVars = append(envVars, &InstanceGroupEnvironmentVariable{
				Name: "(all keys from " + envFrom.ConfigMapRef.Name + ")",
				Source: InstanceGroupValueSource{
					Kind: InstanceGroupValueSourceKindConfigMap,
					Name: envFrom.ConfigMapRef.Name,
				},
			})
		}
	}

	return envVars
}

// ListMountedFiles extracts mounted files (from Secrets/ConfigMaps) from the instance group's pod template.
func ListMountedFiles(ctx context.Context, ig *InstanceGroup) []*InstanceGroupMountedFile {
	if len(ig.PodTemplateSpec.Spec.Containers) == 0 {
		return nil
	}

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

		file := &InstanceGroupMountedFile{
			Path: mount.MountPath,
		}

		switch {
		case vol.Secret != nil:
			file.RequiresElevation = true
			file.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSecret,
				Name: vol.Secret.SecretName,
			}
		case vol.ConfigMap != nil:
			file.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindConfigMap,
				Name: vol.ConfigMap.Name,
			}
		case vol.Projected != nil:
			// Projected volumes can contain multiple sources. List first source as representative.
			file.Source = InstanceGroupValueSource{
				Kind: InstanceGroupValueSourceKindSpec,
				Name: "projected",
			}
			for _, source := range vol.Projected.Sources {
				if source.Secret != nil {
					file.RequiresElevation = true
					file.Source = InstanceGroupValueSource{
						Kind: InstanceGroupValueSourceKindSecret,
						Name: source.Secret.Name,
					}
					break
				}
				if source.ConfigMap != nil {
					file.Source = InstanceGroupValueSource{
						Kind: InstanceGroupValueSourceKindConfigMap,
						Name: source.ConfigMap.Name,
					}
					break
				}
			}
		default:
			// Skip volumes that aren't from secrets/configmaps (emptyDir, hostPath, etc.)
			continue
		}

		files = append(files, file)
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
