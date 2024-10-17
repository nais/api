package application

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/graphv1/ident"
	"github.com/nais/api/internal/v1/graphv1/modelv1"
	"github.com/nais/api/internal/v1/graphv1/pagination"
	"github.com/nais/api/internal/v1/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type (
	ApplicationConnection = pagination.Connection[*Application]
	ApplicationEdge       = pagination.Edge[*Application]
)

type Application struct {
	workload.Base
	Spec *nais_io_v1alpha1.ApplicationSpec `json:"-"`
}

func (Application) IsNode()       {}
func (Application) IsSearchNode() {}
func (Application) IsWorkload()   {}

func (a Application) ID() ident.Ident {
	return newIdent(a.TeamSlug, a.EnvironmentName, a.Name)
}

type Instance struct {
	Name     string    `json:"name"`
	Restarts int       `json:"restarts"`
	Created  time.Time `json:"created"`

	ImageString             string `json:"-"`
	EnvironmentName         string `json:"-"`
	Spec                    *corev1.Pod
	WorkloadContainerStatus corev1.ContainerStatus
}

func toGraphInstance(pod *corev1.Pod, environmentName string, workloadName string) *Instance {
	ret := &Instance{
		Name:            pod.Name,
		Restarts:        int(pod.Status.ContainerStatuses[0].RestartCount),
		Created:         pod.CreationTimestamp.Time,
		Spec:            pod,
		EnvironmentName: environmentName,
		ImageString:     pod.Spec.Containers[0].Image,
	}

	for _, c := range pod.Status.ContainerStatuses {
		if c.Name == workloadName {
			ret.WorkloadContainerStatus = c
			break
		}
	}

	return ret
}

func (i Instance) Image() *workload.ContainerImage {
	name, tag, _ := strings.Cut(i.ImageString, ":")
	return &workload.ContainerImage{
		Name: name,
		Tag:  tag,
	}
}

func (i *Instance) State() InstanceState {
	switch {
	case i.WorkloadContainerStatus.State.Running != nil:
		return InstanceStateRunning
	case i.WorkloadContainerStatus.State.Waiting != nil:
		return InstanceStateFailing
	default:
		return InstanceStateUnknown
	}
}

type ApplicationManifest struct {
	Content string `json:"content"`
}

func (ApplicationManifest) IsWorkloadManifest() {}

type ApplicationOrder struct {
	Field     ApplicationOrderField  `json:"field"`
	Direction modelv1.OrderDirection `json:"direction"`
}

type ApplicationOrderField string

const (
	ApplicationOrderFieldName           ApplicationOrderField = "NAME"
	ApplicationOrderFieldStatus         ApplicationOrderField = "STATUS"
	ApplicationOrderFieldEnvironment    ApplicationOrderField = "ENVIRONMENT"
	ApplicationOrderFieldDeploymentTime ApplicationOrderField = "DEPLOYMENT_TIME"
)

func (e ApplicationOrderField) IsValid() bool {
	switch e {
	case ApplicationOrderFieldStatus, ApplicationOrderFieldName, ApplicationOrderFieldEnvironment, ApplicationOrderFieldDeploymentTime:
		return true
	}
	return false
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
				ret.Limits.CPU = q.AsApproximateFloat64()
			}

			if m, err := resource.ParseQuantity(resources.Limits.Memory); err == nil {
				ret.Limits.Memory = m.Value()
			}
		}

		if resources.Requests != nil {
			if q, err := resource.ParseQuantity(resources.Requests.Cpu); err == nil {
				ret.Requests.CPU = q.AsApproximateFloat64()
			}

			if m, err := resource.ParseQuantity(resources.Requests.Memory); err == nil {
				ret.Requests.Memory = m.Value()
			}
		}

	}

	if replicas := a.Spec.Replicas; replicas != nil {
		if replicas.Min != nil {
			ret.Scaling.MinInstances = *replicas.Min
		}

		if replicas.Max != nil {
			ret.Scaling.MaxInstances = *replicas.Max
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

	return &Application{
		Base: workload.Base{
			Name:            application.Name,
			EnvironmentName: environmentName,
			TeamSlug:        slug.Slug(application.Namespace),
			ImageString:     application.Spec.Image,
			Conditions:      getConditions(application.Status),
			AccessPolicy:    application.Spec.AccessPolicy,
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

type InstanceState string

const (
	InstanceStateRunning InstanceState = "RUNNING"
	InstanceStateFailing InstanceState = "FAILING"
	InstanceStateUnknown InstanceState = "UNKNOWN"
)

var AllInstanceState = []InstanceState{
	InstanceStateRunning,
	InstanceStateFailing,
	InstanceStateUnknown,
}

func (e InstanceState) IsValid() bool {
	switch e {
	case InstanceStateRunning, InstanceStateFailing, InstanceStateUnknown:
		return true
	}
	return false
}

func (e InstanceState) String() string {
	return string(e)
}

func (e *InstanceState) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = InstanceState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid InstanceState", str)
	}
	return nil
}

func (e InstanceState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

type TeamInventoryCountApplications struct {
	// Total number of applications.
	Total int `json:"total"`
	// Number of applications considered not nais.
	NotNais int `json:"notNais"`
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
