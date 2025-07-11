package application

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

type (
	ApplicationConnection         = pagination.Connection[*Application]
	ApplicationEdge               = pagination.Edge[*Application]
	ApplicationInstanceConnection = pagination.Connection[*ApplicationInstance]
	ApplicationInstanceEdge       = pagination.Edge[*ApplicationInstance]
)

type Application struct {
	workload.Base
	Spec *nais_io_v1alpha1.ApplicationSpec `json:"-"`
}

func (Application) IsNode()           {}
func (Application) IsSearchNode()     {}
func (Application) IsWorkload()       {}
func (Application) IsActivityLogger() {}

func (a Application) ID() ident.Ident {
	return newIdent(a.TeamSlug, a.EnvironmentName, a.Name)
}

// GetSecrets returns a list of secret names used by the application
func (a *Application) GetSecrets() []string {
	ret := make([]string, 0)
	for _, v := range a.Spec.EnvFrom {
		ret = append(ret, v.Secret)
	}
	for _, v := range a.Spec.FilesFrom {
		ret = append(ret, v.Secret)
	}
	return ret
}

type ApplicationInstance struct {
	Name     string    `json:"name"`
	Restarts int       `json:"restarts"`
	Created  time.Time `json:"created"`

	ImageString                string                 `json:"-"`
	EnvironmentName            string                 `json:"-"`
	TeamSlug                   slug.Slug              `json:"-"`
	ApplicationName            string                 `json:"-"`
	ApplicationContainerStatus corev1.ContainerStatus `json:"-"`
}

func (ApplicationInstance) IsNode() {}

func (i ApplicationInstance) ID() ident.Ident {
	return newInstanceIdent(i.TeamSlug, i.EnvironmentName, i.ApplicationName, i.Name)
}

func (i *ApplicationInstance) Status() *ApplicationInstanceStatus {
	switch {
	case i.ApplicationContainerStatus.State.Running != nil:
		return &ApplicationInstanceStatus{
			State:   ApplicationInstanceStateRunning,
			Message: "Running",
		}
	case i.ApplicationContainerStatus.State.Terminated != nil:
		return &ApplicationInstanceStatus{
			State:   ApplicationInstanceStateFailing,
			Message: i.ApplicationContainerStatus.State.Terminated.Reason,
		}
	case i.ApplicationContainerStatus.State.Waiting != nil:
		return &ApplicationInstanceStatus{
			State:   ApplicationInstanceStateFailing,
			Message: i.ApplicationContainerStatus.State.Waiting.Reason,
		}
	default:
		return &ApplicationInstanceStatus{
			State:   ApplicationInstanceStateUnknown,
			Message: "Unknown",
		}
	}
}

func toGraphInstance(pod *corev1.Pod, teamSlug slug.Slug, environmentName string, applicationName string) *ApplicationInstance {
	var containerStatus corev1.ContainerStatus
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == applicationName {
			containerStatus = c
			break
		}
	}

	ret := &ApplicationInstance{
		Name:                       pod.Name,
		Restarts:                   int(containerStatus.RestartCount),
		Created:                    pod.CreationTimestamp.Time,
		EnvironmentName:            environmentName,
		ImageString:                pod.Spec.Containers[0].Image,
		TeamSlug:                   teamSlug,
		ApplicationName:            applicationName,
		ApplicationContainerStatus: containerStatus,
	}

	return ret
}

func (i ApplicationInstance) Image() *workload.ContainerImage {
	name, tag, _ := strings.Cut(i.ImageString, ":")
	return &workload.ContainerImage{
		Name: name,
		Tag:  tag,
	}
}

func (i *ApplicationInstance) State() ApplicationInstanceState {
	switch {
	case i.ApplicationContainerStatus.State.Running != nil:
		return ApplicationInstanceStateRunning
	case i.ApplicationContainerStatus.State.Waiting != nil:
		return ApplicationInstanceStateFailing
	default:
		return ApplicationInstanceStateUnknown
	}
}

type ApplicationManifest struct {
	Content string `json:"content"`
}

func (ApplicationManifest) IsWorkloadManifest() {}

type ApplicationOrder struct {
	Field     ApplicationOrderField `json:"field"`
	Direction model.OrderDirection  `json:"direction"`
}

type ApplicationOrderField string

func (e ApplicationOrderField) IsValid() bool {
	return SortFilter.SupportsSort(e)
}

func (e ApplicationOrderField) String() string {
	return string(e)
}

func (e *ApplicationOrderField) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ApplicationOrderField(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ApplicationOrderField", str)
	}
	return nil
}

func (e ApplicationOrderField) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ScalingStrategy interface {
	IsScalingStrategy()
}

type ApplicationResources struct {
	Limits   *workload.WorkloadResourceQuantity `json:"limits"`
	Requests *workload.WorkloadResourceQuantity `json:"requests"`
	Scaling  *ApplicationScaling                `json:"scaling"`
}

func (ApplicationResources) IsWorkloadResources() {}

type ApplicationScaling struct {
	MinInstances int               `json:"minInstances"`
	MaxInstances int               `json:"maxInstances"`
	Strategies   []ScalingStrategy `json:"strategies"`
}

type CPUScalingStrategy struct {
	Threshold int `json:"threshold"`
}

func (CPUScalingStrategy) IsScalingStrategy() {}

type KafkaLagScalingStrategy struct {
	Threshold     int    `json:"threshold"`
	ConsumerGroup string `json:"consumerGroup"`
	TopicName     string `json:"topicName"`
}

func (KafkaLagScalingStrategy) IsScalingStrategy() {}

func (a *Application) Ingresses() []*Ingress {
	ret := make([]*Ingress, len(a.Spec.Ingresses))
	for i, ingress := range a.Spec.Ingresses {
		ret[i] = &Ingress{
			URL:             string(ingress),
			EnvironmentName: a.EnvironmentName,
			TeamSlug:        a.TeamSlug,
			ApplicationName: a.Name,
		}
	}
	return ret
}

func (a *Application) Resources() *ApplicationResources {
	ret := &ApplicationResources{
		Limits:   &workload.WorkloadResourceQuantity{},
		Requests: &workload.WorkloadResourceQuantity{},
		Scaling:  &ApplicationScaling{},
	}

	if resources := a.Spec.Resources; resources != nil {
		if resources.Limits != nil {
			if q, err := resource.ParseQuantity(resources.Limits.Cpu); err == nil {
				ret.Limits.CPU = ptr.To(q.AsApproximateFloat64())
			}

			if m, err := resource.ParseQuantity(resources.Limits.Memory); err == nil {
				ret.Limits.Memory = ptr.To(m.Value())
			}
		}

		if resources.Requests != nil {
			if q, err := resource.ParseQuantity(resources.Requests.Cpu); err == nil {
				ret.Requests.CPU = ptr.To(q.AsApproximateFloat64())
			}

			if m, err := resource.ParseQuantity(resources.Requests.Memory); err == nil {
				ret.Requests.Memory = ptr.To(m.Value())
			}
		}

	}

	if replicas := a.Spec.Replicas; replicas != nil {
		if replicas.Min != nil {
			ret.Scaling.MinInstances = *replicas.Min
		} else {
			ret.Scaling.MinInstances = 2
		}

		if replicas.Max != nil {
			ret.Scaling.MaxInstances = *replicas.Max
		} else {
			ret.Scaling.MaxInstances = 4
		}

		strategy := replicas.ScalingStrategy
		if strategy != nil && strategy.Cpu != nil && strategy.Cpu.ThresholdPercentage > 0 {
			ret.Scaling.Strategies = append(ret.Scaling.Strategies, CPUScalingStrategy{
				Threshold: strategy.Cpu.ThresholdPercentage,
			})
		}

		if strategy != nil && strategy.Kafka != nil && strategy.Kafka.Threshold > 0 {
			ret.Scaling.Strategies = append(ret.Scaling.Strategies, KafkaLagScalingStrategy{
				Threshold:     strategy.Kafka.Threshold,
				ConsumerGroup: strategy.Kafka.ConsumerGroup,
				TopicName:     strategy.Kafka.Topic,
			})
		}

		if len(ret.Scaling.Strategies) == 0 && ret.Scaling.MinInstances != ret.Scaling.MaxInstances {
			ret.Scaling.Strategies = append(ret.Scaling.Strategies, CPUScalingStrategy{
				Threshold: 50,
			})
		}
	} else {
		ret.Scaling.MinInstances = 2
		ret.Scaling.MaxInstances = 4
		ret.Scaling.Strategies = append(ret.Scaling.Strategies, CPUScalingStrategy{
			Threshold: 50,
		})
	}

	return ret
}

func toGraphApplication(application *nais_io_v1alpha1.Application, environmentName string) *Application {
	getConditions := func(status nais_io_v1.Status) []metav1.Condition {
		if status.Conditions == nil {
			return nil
		}

		return *status.Conditions
	}

	var logging *nais_io_v1.Logging
	if application.Spec.Observability != nil && application.Spec.Observability.Logging != nil {
		logging = application.Spec.Observability.Logging
	}

	var deletedAt *time.Time
	if application.DeletionTimestamp != nil {
		deletedAt = ptr.To(application.DeletionTimestamp.Time)
	}

	imageString := application.GetEffectiveImage()
	if len(imageString) == 0 {
		imageString = application.GetImage()
	}

	return &Application{
		Base: workload.Base{
			Name:                application.Name,
			DeletionStartedAt:   deletedAt,
			EnvironmentName:     environmentName,
			TeamSlug:            slug.Slug(application.Namespace),
			ImageString:         imageString,
			Conditions:          getConditions(application.Status),
			AccessPolicy:        application.Spec.AccessPolicy,
			Annotations:         application.GetAnnotations(),
			RolloutCompleteTime: application.GetStatus().RolloutCompleteTime,
			Type:                workload.TypeApplication,
			Logging:             logging,
		},
		Spec: &application.Spec,
	}
}

type DeleteApplicationInput struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

type DeleteApplicationPayload struct {
	TeamSlug *slug.Slug `json:"-"`
	Success  bool       `json:"-"`
}

type RestartApplicationInput struct {
	Name            string    `json:"name"`
	TeamSlug        slug.Slug `json:"teamSlug"`
	EnvironmentName string    `json:"environmentName"`
}

type RestartApplicationPayload struct {
	TeamSlug        slug.Slug `json:"-"`
	ApplicationName string    `json:"-"`
	EnvironmentName string    `json:"-"`
}

type ApplicationInstanceState string

const (
	ApplicationInstanceStateRunning ApplicationInstanceState = "RUNNING"
	ApplicationInstanceStateFailing ApplicationInstanceState = "FAILING"
	ApplicationInstanceStateUnknown ApplicationInstanceState = "UNKNOWN"
)

var AllApplicationInstanceState = []ApplicationInstanceState{
	ApplicationInstanceStateRunning,
	ApplicationInstanceStateFailing,
	ApplicationInstanceStateUnknown,
}

func (e ApplicationInstanceState) IsValid() bool {
	switch e {
	case ApplicationInstanceStateRunning, ApplicationInstanceStateFailing, ApplicationInstanceStateUnknown:
		return true
	}
	return false
}

func (e ApplicationInstanceState) String() string {
	return string(e)
}

func (e *ApplicationInstanceState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ApplicationInstanceState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ApplicationInstanceState", str)
	}
	return nil
}

func (e ApplicationInstanceState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamInventoryCountApplications struct {
	Total    int       `json:"total"`
	TeamSlug slug.Slug `json:"-"`
}

type Ingress struct {
	URL             string    `json:"url"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	ApplicationName string    `json:"-"`
}

type IngressType string

const (
	IngressTypeUnknown       IngressType = "UNKNOWN"
	IngressTypeExternal      IngressType = "EXTERNAL"
	IngressTypeInternal      IngressType = "INTERNAL"
	IngressTypeAuthenticated IngressType = "AUTHENTICATED"
)

var AllIngressType = []IngressType{
	IngressTypeUnknown,
	IngressTypeExternal,
	IngressTypeInternal,
	IngressTypeAuthenticated,
}

func (e IngressType) IsValid() bool {
	switch e {
	case IngressTypeUnknown, IngressTypeExternal, IngressTypeInternal, IngressTypeAuthenticated:
		return true
	}
	return false
}

func (e IngressType) String() string {
	return string(e)
}

func (e *IngressType) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = IngressType(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid IngressType", str)
	}
	return nil
}

func (e IngressType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type ApplicationInstanceStatus struct {
	State   ApplicationInstanceState `json:"state"`
	Message string                   `json:"message"`
}

type TeamApplicationsFilter struct {
	Name         string   `json:"name"`
	Environments []string `json:"environments"`
}

type ScalingDirection string

const (
	// The scaling direction is up.
	ScalingDirectionUp ScalingDirection = "UP"
	// The scaling direction is down.
	ScalingDirectionDown ScalingDirection = "DOWN"
)

var AllScalingDirection = []ScalingDirection{
	ScalingDirectionUp,
	ScalingDirectionDown,
}

func (e ScalingDirection) IsValid() bool {
	switch e {
	case ScalingDirectionUp, ScalingDirectionDown:
		return true
	}
	return false
}

func (e ScalingDirection) String() string {
	return string(e)
}

func (e *ScalingDirection) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ScalingDirection(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ScalingDirection", str)
	}
	return nil
}

func (e ScalingDirection) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
