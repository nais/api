package application

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/nais/api/internal/activitylog"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/search"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

func ListAllForTeam(ctx context.Context, teamSlug slug.Slug) []*Application {
	k8s := fromContext(ctx).appWatcher
	allApplications := k8s.GetByNamespace(teamSlug.String())

	ret := make([]*Application, len(allApplications))
	for i, obj := range allApplications {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster)
	}
	return ret
}

func ListAllForTeamInEnvironment(ctx context.Context, teamSlug slug.Slug, environmentName string) []*Application {
	k8s := fromContext(ctx).appWatcher
	allApplications := k8s.GetByNamespace(teamSlug.String(), watcher.InCluster(environmentName))

	ret := make([]*Application, len(allApplications))
	for i, obj := range allApplications {
		ret[i] = toGraphApplication(obj.Obj, obj.Cluster)
	}
	return ret
}

func Get(ctx context.Context, teamSlug slug.Slug, environment, name string) (*Application, error) {
	a, err := fromContext(ctx).appWatcher.Get(environment, teamSlug.String(), name)
	if err != nil {
		return nil, err
	}
	return toGraphApplication(a, environment), nil
}

func GetByIdent(ctx context.Context, id ident.Ident) (*Application, error) {
	teamSlug, env, name, err := parseIdent(id)
	if err != nil {
		return nil, err
	}
	return Get(ctx, teamSlug, env, name)
}

func Search(ctx context.Context, q string) ([]*search.Result, error) {
	apps := fromContext(ctx).appWatcher.All()

	ret := make([]*search.Result, 0)
	for _, app := range apps {
		rank := search.Match(q, app.Obj.Name)
		if search.Include(rank) {
			ret = append(ret, &search.Result{
				Rank: rank,
				Node: toGraphApplication(app.Obj, app.Cluster),
			})
		}
	}

	return ret, nil
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
		ResourceType:    activityLogEntryResourceTypeApplication,
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
	client, err := fromContext(ctx).appWatcher.ImpersonatedClient(ctx, environmentName, opts...)
	if err != nil {
		return fmt.Errorf("impersonated client: %w", err)
	}

	b := []byte(fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": %q}}}}}`, time.Now().Format(time.RFC3339)))
	if _, err := client.Namespace(teamSlug.String()).Patch(ctx, name, types.MergePatchType, b, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("patch deployment: %w", err)
	}

	return activitylog.Create(ctx, activitylog.CreateInput{
		Action:          activityLogEntryActionRestartApplication,
		ResourceType:    activityLogEntryResourceTypeApplication,
		TeamSlug:        &teamSlug,
		EnvironmentName: &environmentName,
		ResourceName:    name,
		Actor:           authz.ActorFromContext(ctx).User,
	})
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
