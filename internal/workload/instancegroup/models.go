package instancegroup

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/workload"
	"github.com/nais/api/internal/workload/secret"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// InstanceGroup represents a group of identical instances (backed by a ReplicaSet).
// All instances in the group share the same configuration (env vars, mounts, image).
type InstanceGroup struct {
	Name             string    `json:"name"`
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
	// Content is the file content. Null if the value comes from a Secret (requires elevation to view)
	// or if the source could not be resolved.
	Content *string `json:"content"`
	// Encoding indicates how the content is encoded.
	Encoding secret.ValueEncoding `json:"encoding"`
	// Error is set when the source Secret or ConfigMap could not be resolved.
	// When set, the file entry represents a failed mount rather than an actual file.
	Error *string `json:"error"`
}

// InstanceGroupValueSource describes where a value comes from.
type InstanceGroupValueSource struct {
	Kind InstanceGroupValueSourceKind `json:"kind"`
	Name string                       `json:"name"`
}

// InstanceGroupValueSourceKind indicates the type of source for an env var or file.
type InstanceGroupValueSourceKind string

const (
	InstanceGroupValueSourceKindSecret InstanceGroupValueSourceKind = "SECRET"
	InstanceGroupValueSourceKindConfig InstanceGroupValueSourceKind = "CONFIG"
	InstanceGroupValueSourceKindSpec   InstanceGroupValueSourceKind = "SPEC"
)

var AllInstanceGroupValueSourceKind = []InstanceGroupValueSourceKind{
	InstanceGroupValueSourceKindSecret,
	InstanceGroupValueSourceKindConfig,
	InstanceGroupValueSourceKindSpec,
}

func (e InstanceGroupValueSourceKind) IsValid() bool {
	return slices.Contains(AllInstanceGroupValueSourceKind, e)
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

func toGraphInstanceGroup(rs *appsv1.ReplicaSet, environmentName string) *InstanceGroup {
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
