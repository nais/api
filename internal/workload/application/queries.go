package application

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/apierror"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func imageDigestForApplication(l *loaders, appName, namespace, environmentName string) string {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{appName})
	if err != nil {
		return ""
	}
	pods := l.podWatcher.GetByNamespace(namespace, watcher.InCluster(environmentName), watcher.WithLabels(labels.NewSelector().Add(*nameReq)))
	for _, pod := range pods {
		if digest := workload.DigestFromPodStatus(pod.Obj.Spec.Containers, pod.Obj.Status.ContainerStatuses); digest != "" {
			return digest
		}
	}
	return ""
}

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug, orderBy *ApplicationOrder, filter *TeamApplicationsFilter) []*Application {
	l := fromContext(ctx)
	allApplications := l.appWatcher.GetByNamespace(teamSlug.String())
	ret := make([]*Application, len(allApplications))
	for i, obj := range allApplications {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster, imageDigestForApplication(l, obj.Obj.Name, teamSlug.String(), obj.Cluster))
	}

	if filter != nil {
		ret = SortFilter.Filter(ctx, ret, filter)
	}

	if orderBy == nil {
		orderBy = &ApplicationOrder{
			Field:     "NAME",
			Direction: model.OrderDirectionAsc,
		}
	}

	SortFilter.Sort(ctx, ret, orderBy.Field, orderBy.Direction)

	return ret
}

func ListAllInEnvironment(ctx context.Context, environment string) []*Application {
	l := fromContext(ctx)
	apps := l.appWatcher.GetByCluster(environment)
	ret := make([]*Application, len(apps))
	for i, obj := range apps {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster, imageDigestForApplication(l, obj.Obj.Name, obj.Obj.Namespace, obj.Cluster))
	}
	return ret
}

func ListAllForTeamInEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string) []*Application {
	l := fromContext(ctx)
	allApplications := l.appWatcher.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))

	ret := make([]*Application, len(allApplications))
	for i, obj := range allApplications {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster, imageDigestForApplication(l, obj.Obj.Name, teamSlug.String(), obj.Cluster))
	}
	return ret
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Application, error) {
	l := fromContext(ctx)
	a, err := l.appWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphApplication(a, environment, imageDigestForApplication(l, a.Name, teamSlug.String(), environment)), nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Application, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func Manifest(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*ApplicationManifest, error) {
	application, err := fromContext(ctx).appWatcher.Get(environmentName, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}

	manifest := map[string]any{
		"spec":       application.Spec,
		"apiVersion": application.APIVersion,
		"kind":       application.Kind,
		"metadata": map[string]any{
			"labels":    application.GetLabels(),
			"name":      name,
			"namespace": teamSlug.String(),
		},
	}

	b, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, err
	}

	return &ApplicationManifest{
		Content: string(b),
	}, nil
}

func Delete(ctx context.Context, teamSlug slug.Slug, environmentName, name string) (*DeleteApplicationPayload, error) {
	if err := fromContext(ctx).appWatcher.Delete(ctx, environmentName, teamSlug.String(), name); err != nil {
		return nil, err
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionDeleted,
		ResourceType:    ActivityLogEntryResourceTypeApplication,
		TeamSlug:        &teamSlug,
		EnvironmentName: &environmentName,
		ResourceName:    name,
		Actor:           authz.ActorFromContext(ctx).User,
	}); err != nil {
		return nil, err
	}

	return &DeleteApplicationPayload{
		TeamSlug: &teamSlug,
		Success:  true,
	}, nil
}

func Restart(ctx context.Context, teamSlug slug.Slug, environmentName, name string) error {
	opts := []watcher.ImpersonatedClientOption{
		watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{
			Group:    "apps",
			Version:  "v1",
			Resource: "deployments",
		}),
	}
	client, err := fromContext(ctx).appWatcher.SystemAuthenticatedClient(ctx, environmentName, opts...)
	if err != nil {
		return fmt.Errorf("impersonated client: %w", err)
	}

	b := fmt.Appendf(nil, `{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": %q}}}}}`, time.Now().Format(time.RFC3339))
	if _, err := client.Namespace(teamSlug.String()).Patch(ctx, name, types.MergePatchType, b, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patch deployment: %w", err)
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionRestartApplication,
		ResourceType:    ActivityLogEntryResourceTypeApplication,
		TeamSlug:        &teamSlug,
		EnvironmentName: &environmentName,
		ResourceName:    name,
		Actor:           authz.ActorFromContext(ctx).User,
	})
}

func Update(ctx context.Context, input UpdateApplicationInput) (*UpdateApplicationPayload, error) {
	w := fromContext(ctx).appWatcher

	app, err := w.Get(input.EnvironmentName, input.TeamSlug.String(), input.Name)
	if err != nil {
		return nil, err
	}

	var changedFields []*activitylog.ResourceChangedField
	var updateImageResource *string

	if len(input.EnvironmentVariables) > 0 {
		merged := workload.MergeEnvVars(app.Spec.Env, input.EnvironmentVariables)
		if !workload.EnvVarsEqual(app.Spec.Env, merged) {
			changedFields = append(changedFields, workload.EnvVarChangedFields(app.Spec.Env, merged)...)
			app.Spec.Env = merged
		}
	}

	if input.Replicas != nil {
		min, max := input.Replicas.Min, input.Replicas.Max
		if app.Spec.Replicas == nil || !ptr.Equal(app.Spec.Replicas.Min, &min) || !ptr.Equal(app.Spec.Replicas.Max, &max) {
			if app.Spec.Replicas == nil {
				minStr := strconv.Itoa(min)
				maxStr := strconv.Itoa(max)
				changedFields = append(changedFields, &activitylog.ResourceChangedField{Field: "spec.replicas.min", NewValue: &minStr})
				changedFields = append(changedFields, &activitylog.ResourceChangedField{Field: "spec.replicas.max", NewValue: &maxStr})
				app.Spec.Replicas = &nais_io_v1.Replicas{}
			} else {
				if !ptr.Equal(app.Spec.Replicas.Min, &min) {
					var oldMin *string
					if app.Spec.Replicas.Min != nil {
						oldMin = new(strconv.Itoa(*app.Spec.Replicas.Min))
					}
					newMin := strconv.Itoa(min)
					changedFields = append(changedFields, &activitylog.ResourceChangedField{Field: "spec.replicas.min", OldValue: oldMin, NewValue: &newMin})
				}
				if !ptr.Equal(app.Spec.Replicas.Max, &max) {
					var oldMax *string
					if app.Spec.Replicas.Max != nil {
						oldMax = new(strconv.Itoa(*app.Spec.Replicas.Max))
					}
					newMax := strconv.Itoa(max)
					changedFields = append(changedFields, &activitylog.ResourceChangedField{Field: "spec.replicas.max", OldValue: oldMax, NewValue: &newMax})
				}
			}
			app.Spec.Replicas.Min = &min
			app.Spec.Replicas.Max = &max
		}
	}

	if input.Image != nil && *input.Image != "" {
		effectiveImage := app.Status.EffectiveImage
		newImage := *input.Image
		if newImage != effectiveImage {
			if effectiveImage != "" {
				changedFields = append(changedFields, &activitylog.ResourceChangedField{Field: "spec.image", OldValue: &effectiveImage, NewValue: &newImage})
			} else {
				changedFields = append(changedFields, &activitylog.ResourceChangedField{Field: "spec.image", NewValue: &newImage})
			}

			if app.Spec.Image != "" {
				// Image is set directly in the Application spec
				app.Spec.Image = newImage
			} else {
				// Image is managed by a separate Image resource (WorkloadImage pattern)
				updateImageResource = &newImage
			}
		}
	}

	if len(changedFields) == 0 {
		return &UpdateApplicationPayload{
			TeamSlug:        input.TeamSlug,
			EnvironmentName: input.EnvironmentName,
			ApplicationName: input.Name,
		}, nil
	}

	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(app)
	if err != nil {
		return nil, fmt.Errorf("converting application to unstructured: %w", err)
	}

	client, err := w.ImpersonatedClient(ctx, input.EnvironmentName)
	if err != nil {
		return nil, fmt.Errorf("creating impersonated client: %w", err)
	}

	if _, err := client.Namespace(input.TeamSlug.String()).Update(ctx, &unstructured.Unstructured{Object: obj}, metav1.UpdateOptions{}); err != nil {
		return nil, fmt.Errorf("updating application: %w", err)
	}

	if updateImageResource != nil {
		imageClient, err := w.ImpersonatedClient(ctx, input.EnvironmentName, watcher.WithImpersonatedClientGVR(schema.GroupVersionResource{
			Group:    "nais.io",
			Version:  "v1",
			Resource: "images",
		}))
		if err != nil {
			return nil, fmt.Errorf("creating image resource client: %w", err)
		}

		imageObj, err := imageClient.Namespace(input.TeamSlug.String()).Get(ctx, input.Name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil, apierror.Errorf("Image resource %q not found in %s. The application may not be using the image resource pattern.", input.Name, input.EnvironmentName)
			}
			if k8serrors.IsForbidden(err) {
				return nil, apierror.Errorf("You do not have permission to read the image resource for %q.", input.Name)
			}
			return nil, fmt.Errorf("getting image resource: %w", err)
		}

		if err := unstructured.SetNestedField(imageObj.Object, *updateImageResource, "spec", "image"); err != nil {
			return nil, fmt.Errorf("setting image field: %w", err)
		}

		if _, err := imageClient.Namespace(input.TeamSlug.String()).Update(ctx, imageObj, metav1.UpdateOptions{}); err != nil {
			if k8serrors.IsForbidden(err) {
				return nil, apierror.Errorf("You do not have permission to update the image resource for %q.", input.Name)
			}
			if k8serrors.IsConflict(err) {
				return nil, apierror.Errorf("The image resource for %q was modified by another process. Please try again.", input.Name)
			}
			return nil, fmt.Errorf("updating image resource: %w", err)
		}
	}

	if err := activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activitylog.ActivityLogEntryActionUpdated,
		ResourceType:    ActivityLogEntryResourceTypeApplication,
		TeamSlug:        &input.TeamSlug,
		EnvironmentName: &input.EnvironmentName,
		ResourceName:    input.Name,
		Actor:           authz.ActorFromContext(ctx).User,
		Data: &ApplicationUpdatedActivityLogEntryData{
			ChangedFields: changedFields,
		},
	}); err != nil {
		return nil, err
	}

	return &UpdateApplicationPayload{
		TeamSlug:        input.TeamSlug,
		EnvironmentName: input.EnvironmentName,
		ApplicationName: input.Name,
	}, nil
}

func ListInstances(ctx context.Context, teamSlug slug.Slug, environmentName, appName string, page *pagination.Pagination) (*ApplicationInstanceConnection, error) {
	ret, err := ListAllInstances(ctx, teamSlug, environmentName, appName)
	if err != nil {
		return nil, err
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, len(ret)), nil
}

func ListAllInstances(ctx context.Context, teamSlug slug.Slug, environmentName, appName string) ([]*ApplicationInstance, error) {
	pods, err := workload.ListAllPods(ctx, environmentName, teamSlug, appName)
	if err != nil {
		return nil, err
	}

	ret := make([]*ApplicationInstance, len(pods))
	for i, pod := range pods {
		ret[i] = toGraphInstance(pod, teamSlug, environmentName, appName)
	}
	return ret, nil
}

func getInstanceByIdent(ctx context.Context, ident ident.Ident) (*ApplicationInstance, error) {
	teamSlug, env, appName, instanceName, err := parseInstanceIdent(ident)
	if err != nil {
		return nil, err
	}

	pod, err := workload.GetPod(ctx, env, teamSlug, instanceName)
	if err != nil {
		return nil, err
	}

	return toGraphInstance(pod, teamSlug, env, appName), nil
}

func GetIngressType(ctx context.Context, ingress *Ingress) IngressType {
	uri, err := url.Parse(ingress.URL)
	if err != nil {
		return IngressTypeUnknown
	}

	selector := labels.NewSelector()
	req, err := labels.NewRequirement("app", selection.Equals, []string{ingress.ApplicationName})
	if err != nil {
		return IngressTypeUnknown
	}
	selector = selector.Add(*req)

	ings := fromContext(ctx).ingressWatcher.GetByNamespace(ingress.TeamSlug.String(), watcher.WithLabels(selector))
	for _, ing := range ings {
		if ing.Cluster != ingress.EnvironmentName {
			continue
		}

		for _, rule := range ing.Obj.Spec.Rules {
			if rule.Host == uri.Host {
				if ret, ok := ingressClassMapping[ptr.Deref(ing.Obj.Spec.IngressClassName, "")]; ok {
					return ret
				}
				return IngressTypeUnknown
			}
		}
	}

	return IngressTypeUnknown
}

func GetState(ctx context.Context, obj *Application) (ApplicationState, error) {
	i, err := ListAllInstances(ctx, obj.TeamSlug, obj.EnvironmentName, obj.Name)
	if err != nil {
		return 0, fmt.Errorf("listing instances: %w", err)
	}

	if len(i) == 0 {
		return ApplicationStateNotRunning, nil
	}

	for _, instance := range i {
		if instance.State() == ApplicationInstanceStateRunning {
			return ApplicationStateRunning, nil
		}
	}

	return ApplicationStateNotRunning, nil
}

// StateCounts computes the number of applications in each state for a given list of applications.
func StateCounts(ctx context.Context, apps []*Application) (running, notRunning, unknown int) {
	for _, app := range apps {
		state, err := GetState(ctx, app)
		if err != nil {
			unknown++
			continue
		}
		switch state {
		case ApplicationStateRunning:
			running++
		case ApplicationStateNotRunning:
			notRunning++
		default:
			unknown++
		}
	}
	return
}
