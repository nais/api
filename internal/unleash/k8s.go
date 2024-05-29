package unleash

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/nais/api/internal/graph/model"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
)

const (
	UnleashTeamsEnvVar = "TEAMS_ALLOWED_TEAMS"
)

// @TODO decide how we want to specify which team can manage Unleash from Console
func hasAccessToUnleash(team string, unleash *unleash_nais_io_v1.Unleash) bool {
	for _, env := range unleash.Spec.ExtraEnvVars {
		if env.Name == UnleashTeamsEnvVar {
			teams := strings.Split(env.Value, ",")
			for _, t := range teams {
				if t == team {
					return true
				}
			}
		}
	}

	return false
}

func (m *Manager) Unleash(team string) (*model.Unleash, error) {
	for _, informer := range m.mgmCluster.informers {
		objs, err := informer.Lister().List(labels.Everything())
		if err != nil {
			return nil, err
		}

		for _, obj := range objs {
			unleashInstance := &unleash_nais_io_v1.Unleash{}
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.(*unstructured.Unstructured).Object, unleashInstance); err != nil {
				return nil, err
			}

			if hasAccessToUnleash(team, unleashInstance) {
				u := model.ToUnleashInstance(unleashInstance)
				u.Metrics = model.UnleashMetrics{
					CpuRequests:    unleashInstance.Spec.Resources.Requests.Cpu().AsApproximateFloat64(),
					MemoryRequests: unleashInstance.Spec.Resources.Requests.Memory().AsApproximateFloat64(),
					GQLVars: model.UnleashGQLVars{
						Namespace:    m.mgmtNamespace,
						InstanceName: unleashInstance.Name,
					},
				}
				return &model.Unleash{
					Instance: u,
					Enabled:  m.settings.unleashEnabled,
				}, nil
			}
		}
	}
	return &model.Unleash{
		Enabled: m.settings.unleashEnabled,
	}, nil
}
