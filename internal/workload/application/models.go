package application

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/pagination"
	"github.com/nais/api/internal/kubernetes/translate"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ApplicationConnection = pagination.FacetableConnection[*Application, *TeamApplicationsFilter]

type (
	ApplicationEdge               = pagination.Edge[*Application]
	ApplicationInstanceConnection = pagination.Connection[*ApplicationInstance]
	ApplicationInstanceEdge       = pagination.Edge[*ApplicationInstance]
)

type Application struct {
	workload.Base
	Labels []*model.ResourceLabel            `json:"labels"`
	Spec   *nais_io_v1alpha1.ApplicationSpec `json:"-"`
}

func (Application) IsNode()           {}
func (Application) IsSearchNode()     {}
func (Application) IsWorkload()       {}
func (Application) IsActivityLogger() {}

func (a Application) ID() ident.Ident {
	return newIdent(a.TeamSlug, a.EnvironmentName, a.Name)
}

func (a *Application) GetLabels() []*model.ResourceLabel {
	return a.Labels
}

// GetSecrets returns a list of secret names used by the application
func (a *Application) GetSecrets() []string {
	ret := make([]string, 0)
	for _, v := range a.Spec.EnvFrom {
		if v.Secret != "" {
			ret = append(ret, v.Secret)
		}
	}
	for _, v := range a.Spec.FilesFrom {
		if v.Secret != "" {
			ret = append(ret, v.Secret)
		}
	}
	return ret
}

// GetConfigs returns a list of config names used by the application
func (a *Application) GetConfigs() []string {
	ret := make([]string, 0)
	for _, v := range a.Spec.EnvFrom {
		if v.ConfigMap != "" {
			ret = append(ret, v.ConfigMap)
		}
	}
	for _, v := range a.Spec.FilesFrom {
		if v.ConfigMap != "" {
			ret = append(ret, v.ConfigMap)
		}
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
	// SchedulingFailure is the reason the instance cannot be placed on a server, if any.
	SchedulingFailure *string `json:"-"`
	// ComponentContainerStatuses are the statuses of containers added by the platform, including init containers.
	ComponentContainerStatuses []corev1.ContainerStatus `json:"-"`
}

func (ApplicationInstance) IsNode() {}

func (i ApplicationInstance) ID() ident.Ident {
	return newInstanceIdent(i.TeamSlug, i.EnvironmentName, i.ApplicationName, i.Name)
}

// benignWaitingReasons are Kubernetes container waiting reasons that indicate
// normal startup behavior rather than a problem.
var benignWaitingReasons = []string{
	"",
	"ContainerCreating",
	"PodInitializing",
	"Pulling",
	"ImagePull",
}

// benignMessages maps benign waiting reasons to user-friendly messages.
var benignMessages = map[string]string{
	"ContainerCreating": "Container is being created.",
	"PodInitializing":   "Waiting for init containers to complete.",
	"Pulling":           "Downloading container image.",
	"ImagePull":         "Downloading container image.",
}

// problemMessages maps Kubernetes container waiting reasons to user-friendly messages.
// Use %s as a placeholder for the Kubernetes message when it provides useful detail.
var problemMessages = map[string]string{
	"CrashLoopBackOff":           "Instance is crash-looping — it keeps crashing shortly after starting. Check application logs for details.",
	"ErrImagePull":               "Failed to download container image. Check that the image exists and is accessible.",
	"ImagePullBackOff":           "Repeated failures downloading container image. Check that the image exists and is accessible.",
	"CreateContainerConfigError": "Configuration error: %s",
	"CreateContainerError":       "Failed to create the container: %s",
	"RunContainerError":          "Failed to start the container: %s",
	"InvalidImageName":           "Invalid container image name.",
}

// terminatedMessages maps Kubernetes container terminated reasons to user-friendly messages.
var terminatedMessages = map[string]string{
	"OOMKilled": "Instance ran out of memory. Consider increasing the memory limit.",
	"Completed": "Completed successfully.",
}

func classifyWaiting(w *corev1.ContainerStateWaiting) (kind string, msg string) {
	if w == nil {
		return "ok", "not waiting"
	}
	reason := strings.TrimSpace(w.Reason)

	if slices.Contains(benignWaitingReasons, reason) {
		if m, ok := benignMessages[reason]; ok {
			return "benign", m
		}
		return "benign", "Starting."
	}

	if m, ok := problemMessages[reason]; ok {
		if strings.Contains(m, "%s") {
			detail := strings.TrimSpace(w.Message)
			if detail == "" {
				detail = reason
			}
			return "problem", fmt.Sprintf(m, detail)
		}
		return "problem", m
	}

	// Fallback: use the Kubernetes reason as-is
	msg = reason
	if msg == "" {
		msg = strings.TrimSpace(w.Message)
	}
	if msg == "" {
		msg = "Waiting"
	}
	return "problem", msg
}

func terminatedMessage(t *corev1.ContainerStateTerminated) string {
	if m, ok := terminatedMessages[t.Reason]; ok {
		return m
	}
	if t.Reason != "" {
		return fmt.Sprintf("Instance terminated: %s.", t.Reason)
	}
	if t.Message != "" {
		return t.Message
	}
	return fmt.Sprintf("Instance exited with code %d.", t.ExitCode)
}

func (i *ApplicationInstance) lastTerminationInfo() (*string, *int, *time.Time) {
	last := i.ApplicationContainerStatus.State.Terminated
	if last == nil {
		last = i.ApplicationContainerStatus.LastTerminationState.Terminated
	}
	if last == nil {
		return nil, nil, nil
	}
	reason := last.Reason
	exitCode := int(last.ExitCode)
	if reason == "" {
		reason = fmt.Sprintf("ExitCode:%d", exitCode)
	}
	finishedAt := last.FinishedAt.Time
	return &reason, &exitCode, &finishedAt
}

func (i *ApplicationInstance) Status() *ApplicationInstanceStatus {
	lastExitReason, lastExitCode, lastExitTimestamp := i.lastTerminationInfo()
	ready := i.ApplicationContainerStatus.Ready
	current := i.ApplicationContainerStatus.State

	componentProblem, hasComponentProblem := i.ComponentProblem()

	switch {
	case i.SchedulingFailure != nil && current.Running == nil && current.Terminated == nil:
		return &ApplicationInstanceStatus{
			State:             ApplicationInstanceStatePending,
			Message:           translate.SchedulingFailure(*i.SchedulingFailure),
			Ready:             false,
			LastExitReason:    lastExitReason,
			LastExitCode:      lastExitCode,
			LastExitTimestamp: lastExitTimestamp,
		}

	case i.ApplicationContainerStatus.State.Running != nil:
		msg := "Running and ready."
		if hasComponentProblem {
			// The instance does not receive traffic until all its containers are ready.
			ready = false
			msg = "Running, but not receiving traffic. " + componentProblem
		} else if i.hasUnreadyComponent() {
			ready = false
			msg = "Running, but not receiving traffic. A component added by the platform is not ready."
		} else if !ready {
			msg = "Running, but not passing readiness check."
		}
		return &ApplicationInstanceStatus{
			State:             ApplicationInstanceStateRunning,
			Message:           msg,
			Ready:             ready,
			LastExitReason:    lastExitReason,
			LastExitCode:      lastExitCode,
			LastExitTimestamp: lastExitTimestamp,
		}

	case i.ApplicationContainerStatus.State.Terminated != nil:
		t := i.ApplicationContainerStatus.State.Terminated
		msg := terminatedMessage(t)
		state := ApplicationInstanceStateFailing
		if t.ExitCode == 0 {
			state = ApplicationInstanceStateTerminated
		}
		return &ApplicationInstanceStatus{
			State:             state,
			Message:           msg,
			Ready:             false,
			LastExitReason:    lastExitReason,
			LastExitCode:      lastExitCode,
			LastExitTimestamp: lastExitTimestamp,
		}

	case i.ApplicationContainerStatus.State.Waiting != nil:
		kind, msg := classifyWaiting(i.ApplicationContainerStatus.State.Waiting)
		state := ApplicationInstanceStateFailing
		if kind == "benign" {
			state = ApplicationInstanceStateStarting
			if hasComponentProblem {
				state = ApplicationInstanceStateFailing
				msg = componentProblem
			}
		}
		return &ApplicationInstanceStatus{
			State:             state,
			Message:           msg,
			Ready:             false,
			LastExitReason:    lastExitReason,
			LastExitCode:      lastExitCode,
			LastExitTimestamp: lastExitTimestamp,
		}

	case hasComponentProblem:
		return &ApplicationInstanceStatus{
			State:             ApplicationInstanceStateFailing,
			Message:           componentProblem,
			Ready:             false,
			LastExitReason:    lastExitReason,
			LastExitCode:      lastExitCode,
			LastExitTimestamp: lastExitTimestamp,
		}

	default:
		return &ApplicationInstanceStatus{
			State:             ApplicationInstanceStateUnknown,
			Message:           "Unknown state.",
			Ready:             false,
			LastExitReason:    lastExitReason,
			LastExitCode:      lastExitCode,
			LastExitTimestamp: lastExitTimestamp,
		}
	}
}

func (i *ApplicationInstance) State() ApplicationInstanceState {
	return i.Status().State
}

// ToApplicationInstance converts a pod to an ApplicationInstance.
func ToApplicationInstance(pod *corev1.Pod, teamSlug slug.Slug, environmentName string, applicationName string) *ApplicationInstance {
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

	return &ApplicationInstance{
		Name:                       pod.Name,
		Restarts:                   int(containerStatus.RestartCount),
		Created:                    pod.CreationTimestamp.Time,
		EnvironmentName:            environmentName,
		ImageString:                imageString,
		TeamSlug:                   teamSlug,
		ApplicationName:            applicationName,
		ApplicationContainerStatus: containerStatus,
		SchedulingFailure:          schedulingFailure(pod),
		ComponentContainerStatuses: componentStatuses(pod, applicationName),
	}
}

func componentStatuses(pod *corev1.Pod, applicationName string) []corev1.ContainerStatus {
	ret := slices.Clone(pod.Status.InitContainerStatuses)
	for _, c := range pod.Status.ContainerStatuses {
		if c.Name != applicationName {
			ret = append(ret, c)
		}
	}
	return ret
}

func (i *ApplicationInstance) hasUnreadyComponent() bool {
	return slices.ContainsFunc(i.ComponentContainerStatuses, func(c corev1.ContainerStatus) bool {
		if t := c.State.Terminated; t != nil && t.ExitCode == 0 {
			return false
		}
		return !c.Ready
	})
}

// ComponentProblem describes a failing container added by the platform. Container names are not exposed, as these
// containers are not part of the application manifest.
func (i *ApplicationInstance) ComponentProblem() (string, bool) {
	const prefix = "A component added by the platform "
	for _, c := range i.ComponentContainerStatuses {
		if w := c.State.Waiting; w != nil {
			if kind, _ := classifyWaiting(w); kind != "problem" {
				continue
			}
			switch strings.TrimSpace(w.Reason) {
			case "CrashLoopBackOff":
				return prefix + "keeps crashing.", true
			case "ErrImagePull", "ImagePullBackOff", "InvalidImageName":
				return prefix + "could not download its image.", true
			case "CreateContainerConfigError":
				if msg, ok := translate.MissingSource(w.Message); ok {
					return prefix + "is missing configuration. " + msg, true
				}
				return prefix + "is missing configuration.", true
			default:
				return prefix + "could not start.", true
			}
		}
		if t := c.State.Terminated; t != nil && t.ExitCode != 0 {
			if t.Reason == "OOMKilled" {
				return prefix + "ran out of memory.", true
			}
			return prefix + "stopped unexpectedly.", true
		}
	}
	return "", false
}

func schedulingFailure(pod *corev1.Pod) *string {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodScheduled && c.Status == corev1.ConditionFalse && c.Reason == corev1.PodReasonUnschedulable {
			return &c.Message
		}
	}
	return nil
}

func (i ApplicationInstance) Image() *workload.ContainerImage {
	return workload.ParseContainerImage(i.ImageString)
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

func (e *ApplicationOrderField) UnmarshalGQL(v any) error {
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

	TeamSlug        slug.Slug `json:"-"`
	EnvironmentName string    `json:"-"`
	ApplicationName string    `json:"-"`
}

type ApplicationScalingStatus struct {
	CurrentInstances int                     `json:"currentInstances"`
	DesiredInstances int                     `json:"desiredInstances"`
	State            ApplicationScalingState `json:"state"`
	Message          string                  `json:"message"`
}

type ApplicationScalingState string

const (
	ApplicationScalingStateActive             ApplicationScalingState = "ACTIVE"
	ApplicationScalingStateLimitedByMaximum   ApplicationScalingState = "LIMITED_BY_MAXIMUM"
	ApplicationScalingStateMetricsUnavailable ApplicationScalingState = "METRICS_UNAVAILABLE"
	ApplicationScalingStateUnableToScale      ApplicationScalingState = "UNABLE_TO_SCALE"
	ApplicationScalingStateDisabled           ApplicationScalingState = "DISABLED"
)

var AllApplicationScalingState = []ApplicationScalingState{
	ApplicationScalingStateActive,
	ApplicationScalingStateLimitedByMaximum,
	ApplicationScalingStateMetricsUnavailable,
	ApplicationScalingStateUnableToScale,
	ApplicationScalingStateDisabled,
}

func (e ApplicationScalingState) IsValid() bool {
	return slices.Contains(AllApplicationScalingState, e)
}

func (e ApplicationScalingState) String() string {
	return string(e)
}

func (e *ApplicationScalingState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ApplicationScalingState(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ApplicationScalingState", str)
	}
	return nil
}

func (e ApplicationScalingState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// toScalingStatus translates the state of a horizontal pod autoscaler. When several conditions apply, the one that
// is most severe for the user is reported.
func toScalingStatus(hpa *autoscalingv2.HorizontalPodAutoscaler) *ApplicationScalingStatus {
	ret := &ApplicationScalingStatus{
		CurrentInstances: int(hpa.Status.CurrentReplicas),
		DesiredInstances: int(hpa.Status.DesiredReplicas),
		State:            ApplicationScalingStateActive,
		Message:          "Automatic scaling is active.",
	}

	var ableToScale, scalingActive, scalingLimited *autoscalingv2.HorizontalPodAutoscalerCondition
	for i, c := range hpa.Status.Conditions {
		switch c.Type {
		case autoscalingv2.AbleToScale:
			ableToScale = &hpa.Status.Conditions[i]
		case autoscalingv2.ScalingActive:
			scalingActive = &hpa.Status.Conditions[i]
		case autoscalingv2.ScalingLimited:
			scalingLimited = &hpa.Status.Conditions[i]
		}
	}

	switch {
	case ableToScale != nil && ableToScale.Status == corev1.ConditionFalse:
		ret.State = ApplicationScalingStateUnableToScale
		ret.Message = "The platform is currently unable to change the number of instances."
	case scalingActive != nil && scalingActive.Status == corev1.ConditionFalse && scalingActive.Reason == "ScalingDisabled":
		ret.State = ApplicationScalingStateDisabled
		ret.Message = "Automatic scaling is turned off because the application has been scaled down to zero instances."
	case scalingActive != nil && scalingActive.Status == corev1.ConditionFalse:
		ret.State = ApplicationScalingStateMetricsUnavailable
		ret.Message = "The platform cannot read the metrics used for automatic scaling, so the number of instances is not adjusted."
	case scalingLimited != nil && scalingLimited.Status == corev1.ConditionTrue && scalingLimited.Reason == "TooManyReplicas":
		ret.State = ApplicationScalingStateLimitedByMaximum
		ret.Message = "The application needs more instances than the configured maximum. Consider increasing the maximum number of instances or the resources of each instance."
	}

	return ret
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
		Scaling: &ApplicationScaling{
			TeamSlug:        a.TeamSlug,
			EnvironmentName: a.EnvironmentName,
			ApplicationName: a.Name,
		},
	}

	if resources := a.Spec.Resources; resources != nil {
		if resources.Limits != nil {
			if q, err := resource.ParseQuantity(resources.Limits.Cpu); err == nil {
				ret.Limits.CPU = new(q.AsApproximateFloat64())
			}

			if m, err := resource.ParseQuantity(resources.Limits.Memory); err == nil {
				ret.Limits.Memory = new(m.Value())
			}
		}

		if resources.Requests != nil {
			if q, err := resource.ParseQuantity(resources.Requests.Cpu); err == nil {
				ret.Requests.CPU = new(q.AsApproximateFloat64())
			}

			if m, err := resource.ParseQuantity(resources.Requests.Memory); err == nil {
				ret.Requests.Memory = new(m.Value())
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

		if replicas.DisableAutoScaling {
			// We take the easy route for now, just set min and max to min
			ret.Scaling.MaxInstances = ret.Scaling.MinInstances
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
		deletedAt = new(application.DeletionTimestamp.Time)
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
		Labels: model.UserLabels(application.GetLabels()),
		Spec:   &application.Spec,
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

type UpdateApplicationInput struct {
	Name                 string                                             `json:"name"`
	TeamSlug             slug.Slug                                          `json:"teamSlug"`
	EnvironmentName      string                                             `json:"environmentName"`
	EnvironmentVariables []*workload.UpdateWorkloadEnvironmentVariableInput `json:"environmentVariables,omitempty"`
	Replicas             *UpdateApplicationReplicasInput                    `json:"replicas,omitempty"`
	Image                *string                                            `json:"image,omitempty"`
}

type UpdateApplicationReplicasInput struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

type UpdateApplicationPayload struct {
	TeamSlug        slug.Slug `json:"-"`
	ApplicationName string    `json:"-"`
	EnvironmentName string    `json:"-"`
}

type ApplicationInstanceState string

const (
	ApplicationInstanceStateRunning    ApplicationInstanceState = "RUNNING"
	ApplicationInstanceStatePending    ApplicationInstanceState = "PENDING"
	ApplicationInstanceStateStarting   ApplicationInstanceState = "STARTING"
	ApplicationInstanceStateFailing    ApplicationInstanceState = "FAILING"
	ApplicationInstanceStateTerminated ApplicationInstanceState = "TERMINATED"
	ApplicationInstanceStateUnknown    ApplicationInstanceState = "UNKNOWN"
)

var AllApplicationInstanceState = []ApplicationInstanceState{
	ApplicationInstanceStateRunning,
	ApplicationInstanceStatePending,
	ApplicationInstanceStateStarting,
	ApplicationInstanceStateFailing,
	ApplicationInstanceStateTerminated,
	ApplicationInstanceStateUnknown,
}

func (e ApplicationInstanceState) IsValid() bool {
	switch e {
	case ApplicationInstanceStateRunning, ApplicationInstanceStatePending, ApplicationInstanceStateStarting, ApplicationInstanceStateFailing, ApplicationInstanceStateTerminated, ApplicationInstanceStateUnknown:
		return true
	}
	return false
}

func (e ApplicationInstanceState) String() string {
	return string(e)
}

func (e *ApplicationInstanceState) UnmarshalGQL(v any) error {
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
	Total      int       `json:"total"`
	Running    int       `json:"running"`
	NotRunning int       `json:"notRunning"`
	Unknown    int       `json:"unknown"`
	TeamSlug   slug.Slug `json:"-"`
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

func (e *IngressType) UnmarshalGQL(v any) error {
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
	State             ApplicationInstanceState `json:"state"`
	Message           string                   `json:"message"`
	Ready             bool                     `json:"ready"`
	LastExitReason    *string                  `json:"lastExitReason,omitempty"`
	LastExitCode      *int                     `json:"lastExitCode,omitempty"`
	LastExitTimestamp *time.Time               `json:"lastExitTimestamp,omitempty"`
}

type TeamApplicationsFilter struct {
	Name         string               `json:"name"`
	Environments []string             `json:"environments"`
	States       []ApplicationState   `json:"states"`
	Labels       []*model.LabelFilter `json:"labels,omitempty"`
}

type ApplicationFacets struct {
	AllApps []*Application
	Filter  *TeamApplicationsFilter

	filteredOnce sync.Once
	filteredApps []*Application
}

type ApplicationStateFacetItem struct {
	State ApplicationState `json:"state"`
	Count int              `json:"count"`
}

type ScalingDirection string

const (
	ScalingDirectionUp   ScalingDirection = "UP"
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

type ApplicationState int

const (
	ApplicationStateUnknown ApplicationState = iota
	ApplicationStateRunning
	ApplicationStateNotRunning
)

var AllApplicationState = []ApplicationState{
	ApplicationStateUnknown,
	ApplicationStateRunning,
	ApplicationStateNotRunning,
}

func (e ApplicationState) IsValid() bool {
	switch e {
	case ApplicationStateUnknown, ApplicationStateRunning, ApplicationStateNotRunning:
		return true
	}
	return false
}

func (e ApplicationState) String() string {
	switch e {
	case ApplicationStateRunning:
		return "RUNNING"
	case ApplicationStateNotRunning:
		return "NOT_RUNNING"
	}
	return "UNKNOWN"
}

func (e *ApplicationState) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	switch str {
	case "RUNNING":
		*e = ApplicationStateRunning
	case "NOT_RUNNING":
		*e = ApplicationStateNotRunning
	case "UNKNOWN":
		*e = ApplicationStateUnknown
	default:
		return fmt.Errorf("%s is not a valid ApplicationState", str)
	}

	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ApplicationState", str)
	}
	return nil
}

func (e ApplicationState) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}
