package workload

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
)

func TestSplitImage(t *testing.T) {
	tests := []struct {
		image      string
		wantName   string
		wantTag    string
		wantSha    string
		wantHasTag bool
	}{
		{"ghcr.io/org/app:v1@sha256:abc123", "ghcr.io/org/app", "v1", "sha256:abc123", true},
		{"ghcr.io/org/app@sha256:abc123", "ghcr.io/org/app", "latest", "sha256:abc123", false},
		{"ghcr.io/org/app", "ghcr.io/org/app", "latest", "", false},
		{"ghcr.io/org/app:", "ghcr.io/org/app", "latest", "", false},
		{"registry.example.com:5000/org/app:v1", "registry.example.com:5000/org/app", "v1", "", true},
		{"registry.example.com:5000/org/app", "registry.example.com:5000/org/app", "latest", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			gotName, gotTag, gotSha, gotHasTag := SplitImage(tt.image)
			if d := cmp.Diff(
				[]any{tt.wantName, tt.wantTag, tt.wantSha, tt.wantHasTag},
				[]any{gotName, gotTag, gotSha, gotHasTag},
			); d != "" {
				t.Fatalf("SplitImage() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestNewContainerImage(t *testing.T) {
	tests := []struct {
		image   string
		wantRef string
		wantID  string
	}{
		{"ghcr.io/org/app:v1", "ghcr.io/org/app:v1", "ghcr.io/org/app:v1"},
		{"ghcr.io/org/app", "ghcr.io/org/app", "ghcr.io/org/app:"},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			img := NewContainerImage(tt.image)
			if d := cmp.Diff(tt.wantRef, img.Ref()); d != "" {
				t.Fatalf("Ref() mismatch (-want +got):\n%s", d)
			}
			if d := cmp.Diff([]string{tt.wantID}, img.ID().Parts()); d != "" {
				t.Fatalf("ID() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestNewContainerImageWithDigest(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		imageID    string
		wantDigest *string
		wantRef    string
	}{
		{
			name:       "docker-pullable format",
			image:      "ghcr.io/org/app:v1",
			imageID:    "docker-pullable://ghcr.io/org/app@sha256:abc123",
			wantDigest: new("sha256:abc123"),
			wantRef:    "ghcr.io/org/app:v1@sha256:abc123",
		},
		{
			name:       "raw sha256",
			image:      "ghcr.io/org/app:v1",
			imageID:    "sha256:abc123",
			wantDigest: new("sha256:abc123"),
			wantRef:    "ghcr.io/org/app:v1@sha256:abc123",
		},
		{
			name:       "containerd format",
			image:      "ghcr.io/org/app:v1",
			imageID:    "containerd://sha256:abc123",
			wantDigest: new("sha256:abc123"),
			wantRef:    "ghcr.io/org/app:v1@sha256:abc123",
		},
		{
			name:       "image without tag",
			image:      "ghcr.io/org/app@sha256:abc123",
			imageID:    "docker-pullable://ghcr.io/org/app@sha256:abc123",
			wantDigest: new("sha256:abc123"),
			wantRef:    "ghcr.io/org/app@sha256:abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			img := NewContainerImageWithDigest(tt.image, tt.imageID)
			if d := cmp.Diff(tt.wantDigest, img.Digest); d != "" {
				t.Fatalf("Digest mismatch (-want +got):\n%s", d)
			}
			if d := cmp.Diff(tt.wantRef, img.Ref()); d != "" {
				t.Fatalf("Ref() mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestDigestFromPodStatus(t *testing.T) {
	digest := DigestFromPodStatus(
		[]corev1.Container{{Name: "app"}},
		[]corev1.ContainerStatus{
			{Name: "sidecar", ImageID: "docker-pullable://ghcr.io/org/sidecar@sha256:sidecar"},
			{Name: "app", ImageID: "docker-pullable://ghcr.io/org/app@sha256:abc123"},
		},
	)

	if d := cmp.Diff("sha256:abc123", digest); d != "" {
		t.Fatalf("DigestFromPodStatus() mismatch (-want +got):\n%s", d)
	}
}
