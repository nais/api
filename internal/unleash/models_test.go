package unleash

import (
	"context"
	"testing"
	"time"

	"github.com/nais/bifrost/pkg/bifrostclient"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestUpdateUnleashInstanceInput_Validate(t *testing.T) {
	stable := "stable"
	tests := []struct {
		name        string
		input       UpdateUnleashInstanceInput
		wantErr     bool
		errContains string
	}{
		{
			name: "valid with release channel",
			input: UpdateUnleashInstanceInput{
				TeamSlug:       "my-team",
				ReleaseChannel: &stable,
			},
			wantErr: false,
		},
		{
			name: "valid without release channel",
			input: UpdateUnleashInstanceInput{
				TeamSlug: "my-team",
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
			expectedReleaseChannelName: nil,
			expectedAllowedTeams:       []string{"team1", "team2"},
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
			expectedReleaseChannelName: new("stable"),
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

			if tt.expectedReleaseChannelName != nil {
				if result.ReleaseChannelName() == nil {
					t.Error("expected ReleaseChannelName to be set, got nil")
				} else if *result.ReleaseChannelName() != *tt.expectedReleaseChannelName {
					t.Errorf("ReleaseChannelName = %q, want %q", *result.ReleaseChannelName(), *tt.expectedReleaseChannelName)
				}
			} else if result.ReleaseChannelName() != nil {
				t.Errorf("expected ReleaseChannelName to be nil, got %q", *result.ReleaseChannelName())
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

func TestToReleaseChannel(t *testing.T) {
	sequential := "sequential"
	canary := "canary"
	lastUpdatedTime := time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)
	createdAtTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		response bifrostclient.ReleaseChannelResponse
		want     *UnleashReleaseChannel
	}{
		{
			name: "full response",
			response: bifrostclient.ReleaseChannelResponse{
				Name:           "stable",
				CurrentVersion: "5.11.0",
				Type:           &sequential,
				LastUpdated:    &lastUpdatedTime,
				CreatedAt:      createdAtTime,
				Image:          "unleash:5.11.0",
			},
			want: &UnleashReleaseChannel{
				Name:           "stable",
				CurrentVersion: "5.11.0",
				Type:           "sequential",
				LastUpdated:    new(time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC)),
			},
		},
		{
			name: "minimal response",
			response: bifrostclient.ReleaseChannelResponse{
				Name:           "rapid",
				CurrentVersion: "5.12.0",
				Type:           &canary,
				CreatedAt:      createdAtTime,
				Image:          "unleash:5.12.0",
			},
			want: &UnleashReleaseChannel{
				Name:           "rapid",
				CurrentVersion: "5.12.0",
				Type:           "canary",
				LastUpdated:    nil,
			},
		},
		{
			name: "nil type uses empty string",
			response: bifrostclient.ReleaseChannelResponse{
				Name:           "regular",
				CurrentVersion: "5.10.0",
				Type:           nil,
				CreatedAt:      createdAtTime,
				Image:          "unleash:5.10.0",
			},
			want: &UnleashReleaseChannel{
				Name:           "regular",
				CurrentVersion: "5.10.0",
				Type:           "",
				LastUpdated:    nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toReleaseChannel(&tt.response)

			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.CurrentVersion != tt.want.CurrentVersion {
				t.Errorf("CurrentVersion = %q, want %q", got.CurrentVersion, tt.want.CurrentVersion)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %q, want %q", got.Type, tt.want.Type)
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
//
//go:fix inline
func ptr(s string) *string {
	return new(s)
}

//go:fix inline
func timePtr(t time.Time) *time.Time {
	return new(t)
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
