package opensearch_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nais/api/internal/persistence/opensearch"
	"github.com/nais/api/internal/slug"
)

func TestOpenSearchMetadataInput_Validate(t *testing.T) {
	dnsError := "Name must consist of lowercase letters, numbers, and hyphens only. It cannot start or end with a hyphen."

	tests := []struct {
		name    string
		input   opensearch.OpenSearchMetadataInput
		wantErr string
	}{
		{
			name: "valid input",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "my-opensearch",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
		{
			name: "valid input with trimmed whitespace",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "  my-opensearch  ",
				EnvironmentName: "  dev  ",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
		{
			name: "empty name",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: Name must not be empty.\nname: " + dnsError,
		},
		{
			name: "whitespace only name",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "   ",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: Name must not be empty.\nname: " + dnsError,
		},
		{
			name: "invalid DNS name - uppercase",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "MyOpenSearch",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - starts with hyphen",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "-my-opensearch",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - ends with hyphen",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "my-opensearch-",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - contains underscore",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "my_opensearch",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - too long (exceeds 253 chars)",
			input: opensearch.OpenSearchMetadataInput{
				Name:            strings.Repeat("a", 254),
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "empty environment name",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "my-opensearch",
				EnvironmentName: "",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "environmentName: Environment name must not be empty.",
		},
		{
			name: "whitespace only environment name",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "my-opensearch",
				EnvironmentName: "   ",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "environmentName: Environment name must not be empty.",
		},
		{
			name: "empty team slug",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "my-opensearch",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug(""),
			},
			wantErr: "teamSlug: Team slug must not be empty.",
		},
		{
			name: "all fields empty",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "",
				EnvironmentName: "",
				TeamSlug:        slug.Slug(""),
			},
			wantErr: "name: Name must not be empty.\nname: " + dnsError + "\nenvironmentName: Environment name must not be empty.\nteamSlug: Team slug must not be empty.",
		},
		{
			name: "valid DNS name with numbers",
			input: opensearch.OpenSearchMetadataInput{
				Name:            "opensearch-123",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
		{
			name: "valid DNS name - max length (63 chars)",
			input: opensearch.OpenSearchMetadataInput{
				Name:            strings.Repeat("a", 63),
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate(context.Background())
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Error("expected error but got nil")
					return
				}
				if err.Error() != tt.wantErr {
					t.Errorf("error mismatch\ngot:  %q\nwant: %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}
