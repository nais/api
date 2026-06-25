package workload

import (
	"context"
	"testing"
)

func TestParseContainerImageRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		wantName string
		wantTag  string
	}{
		{
			name:     "no tag or digest",
			image:    "registry/repo/app",
			wantName: "registry/repo/app",
			wantTag:  "",
		},
		{
			name:     "tag",
			image:    "registry/repo/app:1.2.3",
			wantName: "registry/repo/app",
			wantTag:  "1.2.3",
		},
		{
			name:     "digest only",
			image:    "registry/repo/app@sha256:abc",
			wantName: "registry/repo/app",
			wantTag:  "sha256:abc",
		},
		{
			name:     "tag and digest",
			image:    "registry/repo/app:1.2.3@sha256:abc",
			wantName: "registry/repo/app",
			wantTag:  "1.2.3@sha256:abc",
		},
		{
			name:     "registry with port and tag",
			image:    "registry.com:443/image/name:tag",
			wantName: "registry.com:443/image/name",
			wantTag:  "tag",
		},
		{
			name:     "registry with port and digest",
			image:    "registry.com:443/image/name@sha256:abc",
			wantName: "registry.com:443/image/name",
			wantTag:  "sha256:abc",
		},
		{
			name:     "registry with port tag and digest",
			image:    "registry.com:443/image/name:tag@sha256:abc",
			wantName: "registry.com:443/image/name",
			wantTag:  "tag@sha256:abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseContainerImage(tt.image)
			if parsed.Name != tt.wantName {
				t.Fatalf("name = %q, want %q", parsed.Name, tt.wantName)
			}

			if parsed.Tag != tt.wantTag {
				t.Fatalf("tag = %q, want %q", parsed.Tag, tt.wantTag)
			}

			if got := parsed.Ref(); got != tt.image {
				t.Fatalf("ref = %q, want %q", got, tt.image)
			}
		})
	}
}

func TestContainerImageIdentRoundTrip(t *testing.T) {
	image := "registry/repo/app@sha256:abc"

	ident := newImageIdent(image)
	parsed, err := getImageByIdent(context.Background(), ident)
	if err != nil {
		t.Fatalf("getImageByIdent returned error: %v", err)
	}

	if got := parsed.Ref(); got != image {
		t.Fatalf("ref = %q, want %q", got, image)
	}
}
