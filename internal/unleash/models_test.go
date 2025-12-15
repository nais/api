package unleash

import (
	"context"
	"testing"
	"time"

	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateUnleashInstanceInput_Validate(t *testing.T) {
	tests := []struct {
		name        string
		input       UpdateUnleashInstanceInput
		wantErr     bool
		errContains string
	}{
		{
			name: "valid with custom version only",
			input: UpdateUnleashInstanceInput{
				TeamSlug:      "my-team",
				CustomVersion: ptr("5.11.0"),
			},
			wantErr: false,
		},
		{
			name: "valid with release channel only",
			input: UpdateUnleashInstanceInput{
				TeamSlug:       "my-team",
				ReleaseChannel: ptr("stable"),
			},
			wantErr: false,
		},
		{
			name: "invalid - both custom version and release channel",
			input: UpdateUnleashInstanceInput{
				TeamSlug:       "my-team",
				CustomVersion:  ptr("5.11.0"),
				ReleaseChannel: ptr("stable"),
			},
			wantErr:     true,
			errContains: "mutually exclusive",
		},
		{
			name: "invalid - neither custom version nor release channel",
			input: UpdateUnleashInstanceInput{
				TeamSlug: "my-team",
			},
			wantErr:     true,
			errContains: "Must specify either",
		},
		{
			name: "invalid - empty strings for both",
			input: UpdateUnleashInstanceInput{
				TeamSlug:       "my-team",
				CustomVersion:  ptr(""),
				ReleaseChannel: ptr(""),
			},
			wantErr:     true,
			errContains: "Must specify either",
		},
		{
			name: "valid - empty custom version but valid release channel",
			input: UpdateUnleashInstanceInput{
				TeamSlug:       "my-team",
				CustomVersion:  ptr(""),
				ReleaseChannel: ptr("stable"),
			},
			wantErr: false,
		},
		{
			name: "valid - empty release channel but valid custom version",
			input: UpdateUnleashInstanceInput{
				TeamSlug:       "my-team",
				CustomVersion:  ptr("5.11.0"),
				ReleaseChannel: ptr(""),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate(context.Background())
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got nil")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestToUnleashInstance(t *testing.T) {
	tests := []struct {
		name                       string
		unleash                    *unleash_nais_io_v1.Unleash
		expectedCustomVersion      *string
		expectedReleaseChannelName *string
		expectedAllowedTeams       []string
		expectedReady              bool
	}{
		{
			name: "basic instance without version tracking",
			unleash: &unleash_nais_io_v1.Unleash{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-team",
				},
				Spec: unleash_nais_io_v1.UnleashSpec{
					WebIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "web.example.com"},
					ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "api.example.com"},
					ExtraEnvVars: []corev1.EnvVar{
						{Name: "TEAMS_ALLOWED_TEAMS", Value: "team1,team2"},
					},
				},
				Status: unleash_nais_io_v1.UnleashStatus{
					Version:    "5.11.0",
					Reconciled: true,
					Connected:  true,
				},
			},
			expectedCustomVersion:      nil,
			expectedReleaseChannelName: nil,
			expectedAllowedTeams:       []string{"team1", "team2"},
			expectedReady:              true,
		},
		{
			name: "instance with custom image",
			unleash: &unleash_nais_io_v1.Unleash{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-team",
				},
				Spec: unleash_nais_io_v1.UnleashSpec{
					CustomImage: "unleash-server:5.12.0",
					WebIngress:  unleash_nais_io_v1.UnleashIngressConfig{Host: "web.example.com"},
					ApiIngress:  unleash_nais_io_v1.UnleashIngressConfig{Host: "api.example.com"},
				},
				Status: unleash_nais_io_v1.UnleashStatus{
					Version:    "5.12.0",
					Reconciled: true,
					Connected:  true,
				},
			},
			expectedCustomVersion:      ptr("5.12.0"),
			expectedReleaseChannelName: nil,
			expectedAllowedTeams:       nil,
			expectedReady:              true,
		},
		{
			name: "instance with release channel",
			unleash: &unleash_nais_io_v1.Unleash{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-team",
				},
				Spec: unleash_nais_io_v1.UnleashSpec{
					ReleaseChannel: unleash_nais_io_v1.UnleashReleaseChannelConfig{
						Name: "stable",
					},
					WebIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "web.example.com"},
					ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "api.example.com"},
				},
				Status: unleash_nais_io_v1.UnleashStatus{
					Version:    "5.11.0",
					Reconciled: true,
					Connected:  true,
				},
			},
			expectedCustomVersion:      nil,
			expectedReleaseChannelName: ptr("stable"),
			expectedAllowedTeams:       nil,
			expectedReady:              true,
		},
		{
			name: "instance not ready - not reconciled",
			unleash: &unleash_nais_io_v1.Unleash{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-team",
				},
				Spec: unleash_nais_io_v1.UnleashSpec{
					WebIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "web.example.com"},
					ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "api.example.com"},
				},
				Status: unleash_nais_io_v1.UnleashStatus{
					Version:    "5.11.0",
					Reconciled: false,
					Connected:  true,
				},
			},
			expectedReady: false,
		},
		{
			name: "instance not ready - not connected",
			unleash: &unleash_nais_io_v1.Unleash{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-team",
				},
				Spec: unleash_nais_io_v1.UnleashSpec{
					WebIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "web.example.com"},
					ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "api.example.com"},
				},
				Status: unleash_nais_io_v1.UnleashStatus{
					Version:    "5.11.0",
					Reconciled: true,
					Connected:  false,
				},
			},
			expectedReady: false,
		},
		{
			name: "allowed teams with empty values filtered",
			unleash: &unleash_nais_io_v1.Unleash{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-team",
				},
				Spec: unleash_nais_io_v1.UnleashSpec{
					WebIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "web.example.com"},
					ApiIngress: unleash_nais_io_v1.UnleashIngressConfig{Host: "api.example.com"},
					ExtraEnvVars: []corev1.EnvVar{
						{Name: "TEAMS_ALLOWED_TEAMS", Value: "team1,,team2,"},
					},
				},
				Status: unleash_nais_io_v1.UnleashStatus{
					Reconciled: true,
					Connected:  true,
				},
			},
			expectedAllowedTeams: []string{"team1", "team2"},
			expectedReady:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toUnleashInstance(tt.unleash)

			if tt.expectedCustomVersion != nil {
				if result.CustomVersion == nil {
					t.Error("expected CustomVersion to be set, got nil")
				} else if *result.CustomVersion != *tt.expectedCustomVersion {
					t.Errorf("CustomVersion = %q, want %q", *result.CustomVersion, *tt.expectedCustomVersion)
				}
			} else if result.CustomVersion != nil {
				t.Errorf("expected CustomVersion to be nil, got %q", *result.CustomVersion)
			}

			if tt.expectedReleaseChannelName != nil {
				if result.ReleaseChannelName == nil {
					t.Error("expected ReleaseChannelName to be set, got nil")
				} else if *result.ReleaseChannelName != *tt.expectedReleaseChannelName {
					t.Errorf("ReleaseChannelName = %q, want %q", *result.ReleaseChannelName, *tt.expectedReleaseChannelName)
				}
			} else if result.ReleaseChannelName != nil {
				t.Errorf("expected ReleaseChannelName to be nil, got %q", *result.ReleaseChannelName)
			}

			if tt.expectedAllowedTeams != nil {
				if len(result.AllowedTeamSlugs) != len(tt.expectedAllowedTeams) {
					t.Errorf("AllowedTeamSlugs length = %d, want %d", len(result.AllowedTeamSlugs), len(tt.expectedAllowedTeams))
				} else {
					for i, team := range tt.expectedAllowedTeams {
						if string(result.AllowedTeamSlugs[i]) != team {
							t.Errorf("AllowedTeamSlugs[%d] = %q, want %q", i, result.AllowedTeamSlugs[i], team)
						}
					}
				}
			}

			if result.Ready != tt.expectedReady {
				t.Errorf("Ready = %v, want %v", result.Ready, tt.expectedReady)
			}
		})
	}
}

func TestBifrostV1ReleaseChannelResponse_ToReleaseChannel(t *testing.T) {
	tests := []struct {
		name     string
		response BifrostV1ReleaseChannelResponse
		want     *UnleashReleaseChannel
	}{
		{
			name: "full response",
			response: BifrostV1ReleaseChannelResponse{
				Name:           "stable",
				CurrentVersion: "5.11.0",
				Type:           "sequential",
				Description:    "Stable release channel",
				LastUpdated:    "2024-03-15T10:30:00Z",
			},
			want: &UnleashReleaseChannel{
				Name:           "stable",
				CurrentVersion: "5.11.0",
				Type:           "sequential",
				Description:    ptr("Stable release channel"),
				LastUpdated:    timePtr(time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)),
			},
		},
		{
			name: "minimal response",
			response: BifrostV1ReleaseChannelResponse{
				Name:           "rapid",
				CurrentVersion: "5.12.0",
				Type:           "canary",
			},
			want: &UnleashReleaseChannel{
				Name:           "rapid",
				CurrentVersion: "5.12.0",
				Type:           "canary",
				Description:    nil,
				LastUpdated:    nil,
			},
		},
		{
			name: "invalid timestamp ignored",
			response: BifrostV1ReleaseChannelResponse{
				Name:           "regular",
				CurrentVersion: "5.10.0",
				Type:           "sequential",
				LastUpdated:    "not-a-valid-timestamp",
			},
			want: &UnleashReleaseChannel{
				Name:           "regular",
				CurrentVersion: "5.10.0",
				Type:           "sequential",
				Description:    nil,
				LastUpdated:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.toReleaseChannel()

			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.CurrentVersion != tt.want.CurrentVersion {
				t.Errorf("CurrentVersion = %q, want %q", got.CurrentVersion, tt.want.CurrentVersion)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
			}

			if tt.want.Description != nil {
				if got.Description == nil {
					t.Error("expected Description to be set, got nil")
				} else if *got.Description != *tt.want.Description {
					t.Errorf("Description = %q, want %q", *got.Description, *tt.want.Description)
				}
			} else if got.Description != nil {
				t.Errorf("expected Description to be nil, got %q", *got.Description)
			}

			if tt.want.LastUpdated != nil {
				if got.LastUpdated == nil {
					t.Error("expected LastUpdated to be set, got nil")
				} else if !got.LastUpdated.Equal(*tt.want.LastUpdated) {
					t.Errorf("LastUpdated = %v, want %v", *got.LastUpdated, *tt.want.LastUpdated)
				}
			} else if got.LastUpdated != nil {
				t.Errorf("expected LastUpdated to be nil, got %v", *got.LastUpdated)
			}
		})
	}
}

// Helper functions
func ptr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStringHelper(s, substr))
}

func containsStringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
