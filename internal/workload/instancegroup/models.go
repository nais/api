package instancegroup

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// InstanceGroup represents a group of identical instances (backed by a ReplicaSet).
// All instances in the group share the same configuration (env vars, mounts, image).
type InstanceGroup struct {
	Name             string    `json:"name"`
	Revision         int       `json:"revision"`
	Created          time.Time `json:"created"`
	ReadyInstances   int       `json:"readyInstances"`
	DesiredInstances int       `json:"desiredInstances"`

	ImageString     string    `json:"-"`
	EnvironmentName string    `json:"-"`
	TeamSlug        slug.Slug `json:"-"`
	ApplicationName string    `json:"-"`

	// PodTemplateSpec holds the pod template from the ReplicaSet for extracting env/mounts.
	PodTemplateSpec corev1.PodTemplateSpec `json:"-"`
}

func (InstanceGroup) IsNode() {}

func (ig InstanceGroup) ID() ident.Ident {
	return newIdent(ig.TeamSlug, ig.EnvironmentName, ig.ApplicationName, ig.Name)
}

func (ig InstanceGroup) Image() *workload.ContainerImage {
	name, tag, _ := strings.Cut(ig.ImageString, ":")
	return &workload.ContainerImage{
		Name: name,
		Tag:  tag,
	}
}

// InstanceGroupEnvironmentVariable represents an environment variable in an instance group.
type InstanceGroupEnvironmentVariable struct {
	Name   string                   `json:"name"`
	Value  *string                  `json:"value"`
	Source InstanceGroupValueSource `json:"source"`
}

// InstanceGroupMountedFile represents a file mounted from a Secret or ConfigMap.
type InstanceGroupMountedFile struct {
	Path   string                   `json:"path"`
	Source InstanceGroupValueSource `json:"source"`
	// Content is the file content. Null for files from Secrets (requires elevation to view).
	// For ConfigMap files, this is the raw string content or base64-encoded binary content.
	Content *string `json:"content"`
	// IsBinary indicates whether the content is base64-encoded binary data.
	IsBinary bool `json:"isBinary"`
}

// InstanceGroupValueSource describes where a value comes from.
type InstanceGroupValueSource struct {
	Kind InstanceGroupValueSourceKind `json:"kind"`
	Name string                       `json:"name"`
}

// InstanceGroupValueSourceKind indicates the type of source for an env var or file.
type InstanceGroupValueSourceKind string

const (
	InstanceGroupValueSourceKindSecret    InstanceGroupValueSourceKind = "SECRET"
	InstanceGroupValueSourceKindConfigMap InstanceGroupValueSourceKind = "CONFIG_MAP"
	InstanceGroupValueSourceKindSpec      InstanceGroupValueSourceKind = "SPEC"
)

var AllInstanceGroupValueSourceKind = []InstanceGroupValueSourceKind{
	InstanceGroupValueSourceKindSecret,
	InstanceGroupValueSourceKindConfigMap,
	InstanceGroupValueSourceKindSpec,
}

func (e InstanceGroupValueSourceKind) IsValid() bool {
	switch e {
	case InstanceGroupValueSourceKindSecret, InstanceGroupValueSourceKindConfigMap, InstanceGroupValueSourceKindSpec:
		return true
	}
	return false
}

func (e InstanceGroupValueSourceKind) String() string {
	return string(e)
}

func (e *InstanceGroupValueSourceKind) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = InstanceGroupValueSourceKind(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid InstanceGroupValueSourceKind", str)
	}
	return nil
}

func (e InstanceGroupValueSourceKind) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

// InstanceGroupEvent represents a translated Kubernetes event.
type InstanceGroupEvent struct {
	Timestamp      time.Time                  `json:"timestamp"`
	Message        string                     `json:"message"`
	Severity       InstanceGroupEventSeverity `json:"severity"`
	SourceInstance *string                    `json:"sourceInstance"`
}

// InstanceGroupEventSeverity indicates the severity of an event.
type InstanceGroupEventSeverity string

const (
	InstanceGroupEventSeverityInfo    InstanceGroupEventSeverity = "INFO"
	InstanceGroupEventSeverityWarning InstanceGroupEventSeverity = "WARNING"
	InstanceGroupEventSeverityError   InstanceGroupEventSeverity = "ERROR"
)

var AllInstanceGroupEventSeverity = []InstanceGroupEventSeverity{
	InstanceGroupEventSeverityInfo,
	InstanceGroupEventSeverityWarning,
	InstanceGroupEventSeverityError,
}

func (e InstanceGroupEventSeverity) IsValid() bool {
	switch e {
	case InstanceGroupEventSeverityInfo, InstanceGroupEventSeverityWarning, InstanceGroupEventSeverityError:
		return true
	}
	return false
}

func (e InstanceGroupEventSeverity) String() string {
	return string(e)
}

func (e *InstanceGroupEventSeverity) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = InstanceGroupEventSeverity(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid InstanceGroupEventSeverity", str)
	}
	return nil
}

func (e InstanceGroupEventSeverity) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(e.String()))
}

func toGraphInstanceGroup(rs *appsv1.ReplicaSet, environmentName string) *InstanceGroup {
	revision := 0
	if v, ok := rs.Annotations["deployment.kubernetes.io/revision"]; ok {
		if parsed, err := strconv.Atoi(v); err == nil {
			revision = parsed
		}
	}

	var desiredInstances int
	if rs.Spec.Replicas != nil {
		desiredInstances = int(*rs.Spec.Replicas)
	}

	var imageString string
	if len(rs.Spec.Template.Spec.Containers) > 0 {
		imageString = rs.Spec.Template.Spec.Containers[0].Image
	}

	appName := rs.Labels["app"]

	return &InstanceGroup{
		Name:             rs.Name,
		Revision:         revision,
		Created:          rs.CreationTimestamp.Time,
		ReadyInstances:   int(rs.Status.ReadyReplicas),
		DesiredInstances: desiredInstances,
		ImageString:      imageString,
		EnvironmentName:  environmentName,
		TeamSlug:         slug.Slug(rs.Namespace),
		ApplicationName:  appName,
		PodTemplateSpec:  rs.Spec.Template,
	}
}
