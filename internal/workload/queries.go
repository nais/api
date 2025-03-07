package workload

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/kubernetes/watcher"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

func getImageByIdent(_ context.Context, id ident.Ident) (*ContainerImage, error) {
	name, err := parseImageIdent(id)
	if err != nil {
		return nil, err
	}

	name, tag, _ := strings.Cut(name, ":")
	return &ContainerImage{
		Name: name,
		Tag:  tag,
	}, nil
}

func GetMaskinPortenAuthIntegration(mp *nais_io_v1.Maskinporten) *MaskinportenAuthIntegration {
	if mp == nil || !mp.Enabled {
		return nil
	}

	return &MaskinportenAuthIntegration{}
}

func GetTokenXAuthIntegration(tx *nais_io_v1.TokenX) *TokenXAuthIntegration {
	if tx == nil || !tx.Enabled {
		return nil
	}

	return &TokenXAuthIntegration{}
}

func GetIDPortenAuthIntegration(idp *nais_io_v1.IDPorten) *IDPortenAuthIntegration {
	if idp == nil || !idp.Enabled {
		return nil
	}

	return &IDPortenAuthIntegration{}
}

func GetEntraIDAuthIntegrationForApplication(azure *nais_io_v1.Azure) *EntraIDAuthIntegration {
	if azure == nil || azure.Application == nil || !azure.Application.Enabled {
		return nil
	}

	return &EntraIDAuthIntegration{}
}

func GetEntraIDAuthIntegrationForJob(azure *nais_io_v1.AzureNaisJob) *EntraIDAuthIntegration {
	if azure == nil || azure.Application == nil || !azure.Application.Enabled {
		return nil
	}

	return &EntraIDAuthIntegration{}
}

func ListAllPods(ctx context.Context, environmentName string, teamSlug slug.Slug, workloadName string) ([]*v1.Pod, error) {
	nameReq, err := labels.NewRequirement("app", selection.Equals, []string{workloadName})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*nameReq)

	pods := fromContext(ctx).podWatcher.GetByNamespace(
		teamSlug.String(),
		watcher.WithLabels(selector),
		watcher.InCluster(environmentName),
	)
	ret := make([]*v1.Pod, len(pods))
	for i, pod := range pods {
		ret[i] = pod.Obj
	}

	return ret, nil
}

func ListAllPodsForJob(ctx context.Context, environmentName string, teamSlug slug.Slug, jobName string) ([]*v1.Pod, error) {
	nameReq, err := labels.NewRequirement("job-name", selection.Equals, []string{jobName})
	if err != nil {
		return nil, err
	}
	selector := labels.NewSelector().Add(*nameReq)

	pods := fromContext(ctx).podWatcher.GetByNamespace(
		teamSlug.String(),
		watcher.WithLabels(selector),
		watcher.InCluster(environmentName),
	)
	ret := make([]*v1.Pod, len(pods))
	for i, pod := range pods {
		ret[i] = pod.Obj
	}

	return ret, nil
}

func GetPod(ctx context.Context, environmentName string, teamSlug slug.Slug, instanceName string) (*v1.Pod, error) {
	return fromContext(ctx).podWatcher.Get(environmentName, teamSlug.String(), instanceName)
}
