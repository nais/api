package valkey_test

import (
	"context"
	"strings"
	"testing"

	"github.com/nais/api/internal/persistence/valkey"
	"github.com/nais/api/internal/slug"
)

func TestValkeyMetadataInput_Validate(t *testing.T) {
	dnsError := "Name must consist of lowercase letters, numbers, and hyphens only. It cannot start or end with a hyphen."

	tests := []struct {
		name    string
		input   valkey.ValkeyMetadataInput
		wantErr string
	}{
		{
			name: "valid input",
			input: valkey.ValkeyMetadataInput{
				Name:            "my-valkey",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
		{
			name: "valid input with trimmed whitespace",
			input: valkey.ValkeyMetadataInput{
				Name:            "  my-valkey  ",
				EnvironmentName: "  dev  ",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
		{
			name: "empty name",
			input: valkey.ValkeyMetadataInput{
				Name:            "",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: Name must not be empty.\nname: " + dnsError,
		},
		{
			name: "whitespace only name",
			input: valkey.ValkeyMetadataInput{
				Name:            "   ",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: Name must not be empty.\nname: " + dnsError,
		},
		{
			name: "invalid DNS name - uppercase",
			input: valkey.ValkeyMetadataInput{
				Name:            "My-Valkey",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - starts with hyphen",
			input: valkey.ValkeyMetadataInput{
				Name:            "-my-valkey",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - ends with hyphen",
			input: valkey.ValkeyMetadataInput{
				Name:            "my-valkey-",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - contains underscore",
			input: valkey.ValkeyMetadataInput{
				Name:            "my_valkey",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "invalid DNS name - too long",
			input: valkey.ValkeyMetadataInput{
				Name:            strings.Repeat("a", 254),
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "name: " + dnsError,
		},
		{
			name: "empty environment name",
			input: valkey.ValkeyMetadataInput{
				Name:            "my-valkey",
				EnvironmentName: "",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "environmentName: Environment name must not be empty.",
		},
		{
			name: "whitespace only environment name",
			input: valkey.ValkeyMetadataInput{
				Name:            "my-valkey",
				EnvironmentName: "   ",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "environmentName: Environment name must not be empty.",
		},
		{
			name: "empty team slug",
			input: valkey.ValkeyMetadataInput{
				Name:            "my-valkey",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug(""),
			},
			wantErr: "teamSlug: Team slug must not be empty.",
		},
		{
			name: "multiple validation errors",
			input: valkey.ValkeyMetadataInput{
				Name:            "",
				EnvironmentName: "",
				TeamSlug:        slug.Slug(""),
			},
			wantErr: "name: Name must not be empty.\nname: " + dnsError + "\nenvironmentName: Environment name must not be empty.\nteamSlug: Team slug must not be empty.",
		},
		{
			name: "valid DNS name with numbers",
			input: valkey.ValkeyMetadataInput{
				Name:            "valkey-123",
				EnvironmentName: "dev",
				TeamSlug:        slug.Slug("my-team"),
			},
			wantErr: "",
		},
		{
			name: "valid DNS name starting with number",
			input: valkey.ValkeyMetadataInput{
				Name:            "123-valkey",
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
