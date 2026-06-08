package application

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestToGraphInstanceUsesApplicationContainerImage(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "pod-1",
			CreationTimestamp: metav1.NewTime(time.Unix(123, 0)),
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "sidecar", Image: "ghcr.io/org/sidecar:v2"},
				{Name: "my-app", Image: "ghcr.io/org/app:v1"},
			},
		},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{Name: "sidecar", ImageID: "containerd://sha256:sidecar", RestartCount: 1},
				{Name: "my-app", ImageID: "containerd://sha256:abc123", RestartCount: 2},
			},
		},
	}

	instance := toGraphInstance(pod, "my-team", "dev", "my-app")

	if instance.ImageString != "ghcr.io/org/app:v1" {
		t.Fatalf("ImageString = %q, want %q", instance.ImageString, "ghcr.io/org/app:v1")
	}
	if instance.ApplicationContainerStatus.Name != "my-app" {
		t.Fatalf("ApplicationContainerStatus.Name = %q, want %q", instance.ApplicationContainerStatus.Name, "my-app")
	}
	if got := instance.Image().Ref(); got != "ghcr.io/org/app:v1@sha256:abc123" {
		t.Fatalf("Image().Ref() = %q, want %q", got, "ghcr.io/org/app:v1@sha256:abc123")
	}
}
