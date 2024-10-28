package application

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/kubernetes/watcher"
	"github.com/nais/api/internal/v1/searchv1"
	"github.com/nais/api/internal/v1/workload"
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

func Search(ctx context.Context, q string) ([]*searchv1.Result, error) {
	apps := fromContext(ctx).appWatcher.All()

	ret := make([]*searchv1.Result, 0)
	for _, app := range apps {
		rank := searchv1.Match(q, app.Obj.Name)
		if searchv1.Include(rank) {
			ret = append(ret, &searchv1.Result{
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
	err := fromContext(ctx).appWatcher.Delete(ctx, environmentName, teamSlug.String(), name)
	if err != nil {
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
		return err
	}

	// depls, err := client.List(ctx, metav1.ListOptions{})
	// if err != nil {
	// 	return err
	// }

	b := []byte(fmt.Sprintf(`{"spec": {"template": {"metadata": {"annotations": {"kubectl.kubernetes.io/restartedAt": "%s"}}}}}`, time.Now().Format(time.RFC3339)))
	if _, err := client.Namespace(teamSlug.String()).Patch(ctx, name, types.StrategicMergePatchType, b, metav1.PatchOptions{}); err != nil {
		return err
	}

	return nil
}

func ListInstances(ctx context.Context, teamSlug slug.Slug, environmentName, appName string, page *pagination.Pagination) (*ApplicationInstanceConnection, error) {
	ret, err := ListAllInstances(ctx, teamSlug, environmentName, appName)
	if err != nil {
		return nil, err
	}

	apps := pagination.Slice(ret, page)
	return pagination.NewConnection(apps, page, int32(len(ret))), nil
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
