package k8s

import (
	"reflect"
	"testing"

	sync_states "github.com/nais/liberator/pkg/events"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/nais/api/internal/graph/model"
)

type testCase struct {
	name           string
	appCondition   string
	instanceStates []model.InstanceState
	image          string
	ingresses      []string
	expectedState  model.State
	expectedErrors []model.StateError
}

func TestSetStatus(t *testing.T) {
	testCases := []testCase{
		{
			name:           "app is rolloutcomplete and has running instances",
			appCondition:   sync_states.RolloutComplete,
			instanceStates: []model.InstanceState{model.InstanceStateRunning},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateNais,
			expectedErrors: []model.StateError{},
		},
		{
			name:           "app is rolloutcomplete and has failing instances",
			appCondition:   sync_states.RolloutComplete,
			instanceStates: []model.InstanceState{model.InstanceStateFailing},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateFailing,
			expectedErrors: []model.StateError{
				&model.NoRunningInstancesError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				},
			},
		},
		{
			name:           "app failed synchronization and has running instances",
			appCondition:   sync_states.FailedSynchronization,
			instanceStates: []model.InstanceState{model.InstanceStateRunning},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateNotnais,
			expectedErrors: []model.StateError{
				&model.InvalidNaisYamlError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				},
			},
		},
		{
			name:           "app failed synchronization and has failing instances",
			appCondition:   sync_states.FailedSynchronization,
			instanceStates: []model.InstanceState{model.InstanceStateFailing},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateFailing,
			expectedErrors: []model.StateError{
				&model.NoRunningInstancesError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				}, &model.InvalidNaisYamlError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				},
			},
		},
		{
			name:           "app is synchronized and has running and failing instances",
			appCondition:   sync_states.Synchronized,
			instanceStates: []model.InstanceState{model.InstanceStateRunning, model.InstanceStateFailing},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateNotnais,
			expectedErrors: []model.StateError{
				&model.NewInstancesFailingError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				},
			},
		},
		{
			name:           "app is synchronized and has multiple failing instances",
			appCondition:   sync_states.Synchronized,
			instanceStates: []model.InstanceState{model.InstanceStateFailing, model.InstanceStateFailing},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateFailing,
			expectedErrors: []model.StateError{
				&model.NewInstancesFailingError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				}, &model.NoRunningInstancesError{
					Revision: "1",
					Level:    model.ErrorLevelError,
				},
			},
		},
		{
			name:           "app is rolloutcomplete and has no instances",
			appCondition:   sync_states.RolloutComplete,
			instanceStates: []model.InstanceState{},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateFailing,
			expectedErrors: []model.StateError{&model.NoRunningInstancesError{
				Revision: "1",
				Level:    model.ErrorLevelError,
			}},
		},
		{
			name:           "app image is from deprecated registry",
			appCondition:   sync_states.RolloutComplete,
			instanceStates: []model.InstanceState{model.InstanceStateRunning, model.InstanceStateRunning},
			image:          "docker.pkg.github.com/nais/myapp/myapp:1.0.0",
			ingresses:      []string{"myapp.nav.cloud.nais.io"},
			expectedState:  model.StateNotnais,
			expectedErrors: []model.StateError{&model.DeprecatedRegistryError{
				Revision: "1",
				Level:    model.ErrorLevelWarning,
			}},
		},
		{
			name:           "app has deprecated ingress",
			appCondition:   sync_states.RolloutComplete,
			instanceStates: []model.InstanceState{model.InstanceStateRunning, model.InstanceStateRunning},
			image:          "europe-north1-docker.pkg.dev/nais-io/nais/images/myapp:1.0.0",
			ingresses:      []string{"myapp.prod-gcp.nais.io"},
			expectedState:  model.StateNotnais,
			expectedErrors: []model.StateError{&model.DeprecatedIngressError{
				Revision: "1",
				Level:    model.ErrorLevelWarning,
			}},
		},
	}

	for _, tc := range testCases {
		app := &model.App{Image: tc.image, Ingresses: tc.ingresses, Env: model.Env{Name: "prod-gcp"}, AutoScaling: model.AutoScaling{Min: 1, Max: 2}}
		setStatus(app, []metav1.Condition{{Status: metav1.ConditionTrue, Reason: string(tc.appCondition)}}, asInstances(tc.instanceStates))

		if app.AppState.State != tc.expectedState {
			t.Errorf("%s\ngot state: %v, want: %v", tc.name, app.AppState.State, tc.expectedState)
		}
		if !hasError(app.AppState.Errors, tc.expectedErrors) {
			t.Errorf("%s\ngot error: %v, want: %v", tc.name, app.AppState.Errors, tc.expectedErrors)
		}
	}
}

func hasError(errors []model.StateError, expectedErrors []model.StateError) bool {
	if len(errors) != len(expectedErrors) {
		return false
	}

	for _, error := range expectedErrors {
		if !contains(errors, error) {
			return false
		}
	}
	return true
}

func contains(slice []model.StateError, s any) bool {
	for _, item := range slice {
		if reflect.TypeOf(item) == reflect.TypeOf(s) {
			return true
		}
	}
	return false
}

func asInstances(states []model.InstanceState) []*model.Instance {
	var ret []*model.Instance
	for _, state := range states {
		ret = append(ret, &model.Instance{State: state})
	}
	return ret
}
