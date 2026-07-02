package workload

import (
	"context"
	"testing"
)

func stringPtr(s string) *string { return &s }

func TestParseContainerImageRoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		image      string
		wantName   string
		wantTag    string
		wantDigest *string
	}{
		{
			name:       "empty string",
			image:      "",
			wantName:   "",
			wantTag:    "",
			wantDigest: nil,
		},
		{
			name:       "simple image name only",
			image:      "nginx",
			wantName:   "nginx",
			wantTag:    "",
			wantDigest: nil,
		},
		{
			name:       "simple image name with tag",
			image:      "nginx:1.25",
			wantName:   "nginx",
			wantTag:    "1.25",
			wantDigest: nil,
		},
		{
			name:       "no tag or digest",
			image:      "registry/repo/app",
			wantName:   "registry/repo/app",
			wantTag:    "",
			wantDigest: nil,
		},
		{
			name:       "tag",
			image:      "registry/repo/app:1.2.3",
			wantName:   "registry/repo/app",
			wantTag:    "1.2.3",
			wantDigest: nil,
		},
		{
			name:       "digest only",
			image:      "registry/repo/app@sha256:abc",
			wantName:   "registry/repo/app",
			wantTag:    "",
			wantDigest: stringPtr("sha256:abc"),
		},
		{
			name:       "tag and digest",
			image:      "registry/repo/app:1.2.3@sha256:abc",
			wantName:   "registry/repo/app",
			wantTag:    "1.2.3",
			wantDigest: stringPtr("sha256:abc"),
		},
		{
			name:       "explicit latest and digest",
			image:      "registry/repo/app:latest@sha256:abc",
			wantName:   "registry/repo/app",
			wantTag:    "latest",
			wantDigest: stringPtr("sha256:abc"),
		},
		{
			name:       "nested path no tag",
			image:      "europe-north1-docker.pkg.dev/my-project/my-repo/my-app",
			wantName:   "europe-north1-docker.pkg.dev/my-project/my-repo/my-app",
			wantTag:    "",
			wantDigest: nil,
		},
		{
			name:       "nested path tag",
			image:      "europe-north1-docker.pkg.dev/my-project/my-repo/my-app:v2.0.0",
			wantName:   "europe-north1-docker.pkg.dev/my-project/my-repo/my-app",
			wantTag:    "v2.0.0",
			wantDigest: nil,
		},
		{
			name:       "nested path digest only",
			image:      "europe-north1-docker.pkg.dev/my-project/my-repo/my-app@sha256:deadbeef",
			wantName:   "europe-north1-docker.pkg.dev/my-project/my-repo/my-app",
			wantTag:    "",
			wantDigest: stringPtr("sha256:deadbeef"),
		},
		{
			name:       "nested path tag and digest",
			image:      "europe-north1-docker.pkg.dev/my-project/my-repo/my-app:v2.0.0@sha256:deadbeef",
			wantName:   "europe-north1-docker.pkg.dev/my-project/my-repo/my-app",
			wantTag:    "v2.0.0",
			wantDigest: stringPtr("sha256:deadbeef"),
		},
		{
			name:       "registry with port and tag",
			image:      "registry.com:443/image/name:tag",
			wantName:   "registry.com:443/image/name",
			wantTag:    "tag",
			wantDigest: nil,
		},
		{
			name:       "registry with port and digest",
			image:      "registry.com:443/image/name@sha256:abc",
			wantName:   "registry.com:443/image/name",
			wantTag:    "",
			wantDigest: stringPtr("sha256:abc"),
		},
		{
			name:       "registry with port and no tag",
			image:      "registry.com:443/image/name",
			wantName:   "registry.com:443/image/name",
			wantTag:    "",
			wantDigest: nil,
		},
		{
			name:       "registry with port nested path and tag",
			image:      "myregistry.io:5000/org/repo/myimage:v2.0.0",
			wantName:   "myregistry.io:5000/org/repo/myimage",
			wantTag:    "v2.0.0",
			wantDigest: nil,
		},
		{
			name:       "registry with port tag and digest",
			image:      "registry.com:443/image/name:tag@sha256:abc",
			wantName:   "registry.com:443/image/name",
			wantTag:    "tag",
			wantDigest: stringPtr("sha256:abc"),
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

			if (parsed.Digest == nil) != (tt.wantDigest == nil) {
				t.Fatalf("digest nil mismatch = %v, want %v", parsed.Digest == nil, tt.wantDigest == nil)
			}
			if parsed.Digest != nil && *parsed.Digest != *tt.wantDigest {
				t.Fatalf("digest = %q, want %q", *parsed.Digest, *tt.wantDigest)
			}

			if got := parsed.Ref(); got != tt.image {
				t.Fatalf("ref = %q, want %q", got, tt.image)
			}
		})
	}
}

func TestParseImageReferenceMetadata(t *testing.T) {
	tests := []struct {
		name               string
		image              string
		wantName           string
		wantTag            string
		wantDigest         string
		wantHasExplicitTag bool
	}{
		{
			name:               "empty string",
			image:              "",
			wantName:           "",
			wantTag:            "",
			wantDigest:         "",
			wantHasExplicitTag: false,
		},
		{
			name:               "image with no tag",
			image:              "registry/repo/app",
			wantName:           "registry/repo/app",
			wantTag:            "",
			wantDigest:         "",
			wantHasExplicitTag: false,
		},
		{
			name:               "image with digest only",
			image:              "registry/repo/app@sha256:abc",
			wantName:           "registry/repo/app",
			wantTag:            "",
			wantDigest:         "sha256:abc",
			wantHasExplicitTag: false,
		},
		{
			name:               "image with explicit latest and digest",
			image:              "registry/repo/app:latest@sha256:abc",
			wantName:           "registry/repo/app",
			wantTag:            "latest",
			wantDigest:         "sha256:abc",
			wantHasExplicitTag: true,
		},
		{
			name:               "registry with port nested path and tag",
			image:              "myregistry.io:5000/org/repo/myimage:v2.0.0",
			wantName:           "myregistry.io:5000/org/repo/myimage",
			wantTag:            "v2.0.0",
			wantDigest:         "",
			wantHasExplicitTag: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed := ParseImageReference(tt.image)
			if parsed.Name != tt.wantName {
				t.Fatalf("name = %q, want %q", parsed.Name, tt.wantName)
			}
			if parsed.Tag != tt.wantTag {
				t.Fatalf("tag = %q, want %q", parsed.Tag, tt.wantTag)
			}
			if parsed.Digest != tt.wantDigest {
				t.Fatalf("digest = %q, want %q", parsed.Digest, tt.wantDigest)
			}
			if parsed.HasExplicitTag != tt.wantHasExplicitTag {
				t.Fatalf("hasExplicitTag = %t, want %t", parsed.HasExplicitTag, tt.wantHasExplicitTag)
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
