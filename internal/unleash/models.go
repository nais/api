package unleash

import (
	"context"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/validate"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type UnleashInstance struct {
	Name       string                  `json:"name"`
	Version    string                  `json:"version"`
	WebIngress string                  `json:"webIngress"`
	APIIngress string                  `json:"apiIngress"`
	Metrics    *UnleashInstanceMetrics `json:"metrics"`
	Ready      bool                    `json:"ready"`

	// Version source tracking (for future release channel support)
	CustomVersion      *string `json:"customVersion,omitempty"`
	ReleaseChannelName *string `json:"releaseChannelName,omitempty"`

	TeamSlug         slug.Slug   `json:"-"`
	AllowedTeamSlugs []slug.Slug `json:"-"`
}

func toUnleashInstance(u *unleash_nais_io_v1.Unleash) *UnleashInstance {
	var teams []slug.Slug
	for _, env := range u.Spec.ExtraEnvVars {
		if env.Name == "TEAMS_ALLOWED_TEAMS" {
			parts := strings.Split(env.Value, ",")
			for _, t := range parts {
				if t == "" {
					continue
				}
				teams = append(teams, slug.Slug(t))
			}
			break
		}
	}

	instance := &UnleashInstance{
		Name:             u.Name,
		Version:          u.Status.Version,
		AllowedTeamSlugs: teams,
		WebIngress:       u.Spec.WebIngress.Host,
		APIIngress:       u.Spec.ApiIngress.Host,
		Ready:            u.Status.Reconciled && u.Status.Connected,
		Metrics: &UnleashInstanceMetrics{
			CPURequests:    u.Spec.Resources.Requests.Cpu().AsApproximateFloat64(),
			MemoryRequests: u.Spec.Resources.Requests.Memory().AsApproximateFloat64(),
			TeamSlug:       slug.Slug(u.Name),
		},
	}

	// Populate version source tracking fields
	if u.Spec.CustomImage != "" {
		// Extract version from custom image
		parts := strings.Split(u.Spec.CustomImage, ":")
		if len(parts) > 1 {
			instance.CustomVersion = &parts[1]
		}
	}

	if u.Spec.ReleaseChannel.Name != "" {
		instance.ReleaseChannelName = &u.Spec.ReleaseChannel.Name
	}

	return instance
}

func (u UnleashInstance) ID() ident.Ident {
	return newUnleashIdent(u.TeamSlug, u.Name)
}

func (UnleashInstance) IsNode() {}

func (u *UnleashInstance) DeepCopyObject() runtime.Object { return nil }

func (u *UnleashInstance) GetName() string                  { return u.Name }
func (u *UnleashInstance) GetNamespace() string             { return ManagementClusterNamespace }
func (u *UnleashInstance) GetLabels() map[string]string     { return nil }
func (u *UnleashInstance) GetObjectKind() schema.ObjectKind { return schema.EmptyObjectKind }

type UnleashInstanceMetrics struct {
	CPURequests    float64 `json:"cpuRequests"`
	MemoryRequests float64 `json:"memoryRequests"`

	TeamSlug slug.Slug `json:"-"`
}

type AllowTeamAccessToUnleashInput struct {
	TeamSlug        slug.Slug `json:"team"`
	AllowedTeamSlug slug.Slug `json:"allowedTeam"`
}

func (i *AllowTeamAccessToUnleashInput) Validate(ctx context.Context) error {
	verr := validate.New()

	_, err := team.Get(ctx, i.AllowedTeamSlug)
	if err != nil {
		verr.Add("allowedTeam", "This team does not exist.")
	}

	return verr.NilIfEmpty()
}

type AllowTeamAccessToUnleashPayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}

type CreateUnleashForTeamInput struct {
	TeamSlug       slug.Slug `json:"team"`
	CustomVersion  *string   `json:"customVersion,omitempty"`
	ReleaseChannel *string   `json:"releaseChannel,omitempty"`
}

func (i *CreateUnleashForTeamInput) Validate(_ context.Context) error {
	verr := validate.New()

	if i.CustomVersion != nil && i.ReleaseChannel != nil && *i.CustomVersion != "" && *i.ReleaseChannel != "" {
		verr.Add("customVersion", "Cannot specify both customVersion and releaseChannel. These options are mutually exclusive.")
		verr.Add("releaseChannel", "Cannot specify both customVersion and releaseChannel. These options are mutually exclusive.")
	}

	return verr.NilIfEmpty()
}

type CreateUnleashForTeamPayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}

type UpdateUnleashInstanceInput struct {
	TeamSlug       slug.Slug `json:"teamSlug"`
	CustomVersion  *string   `json:"customVersion,omitempty"`
	ReleaseChannel *string   `json:"releaseChannel,omitempty"`
}

func (i *UpdateUnleashInstanceInput) Validate(_ context.Context) error {
	verr := validate.New()

	if i.CustomVersion != nil && i.ReleaseChannel != nil && *i.CustomVersion != "" && *i.ReleaseChannel != "" {
		verr.Add("customVersion", "Cannot specify both customVersion and releaseChannel. These options are mutually exclusive.")
		verr.Add("releaseChannel", "Cannot specify both customVersion and releaseChannel. These options are mutually exclusive.")
	}

	// At least one must be specified for an update
	if (i.CustomVersion == nil || *i.CustomVersion == "") && (i.ReleaseChannel == nil || *i.ReleaseChannel == "") {
		verr.Add("customVersion", "Must specify either customVersion or releaseChannel.")
	}

	return verr.NilIfEmpty()
}

type UpdateUnleashInstancePayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}

type RevokeTeamAccessToUnleashInput struct {
	TeamSlug        slug.Slug `json:"team"`
	RevokedTeamSlug slug.Slug `json:"revokedTeam"`
}

type RevokeTeamAccessToUnleashPayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}

// BifrostV1CreateRequest represents the v1 API request format for creating an unleash instance
type BifrostV1CreateRequest struct {
	Name                      string `json:"name"`
	EnableFederation          bool   `json:"enable_federation"`
	AllowedTeams              string `json:"allowed_teams"`
	AllowedClusters           string `json:"allowed_clusters"`
	LogLevel                  string `json:"log_level,omitempty"`
	DatabasePoolMax           int    `json:"database_pool_max,omitempty"`
	DatabasePoolIdleTimeoutMs int    `json:"database_pool_idle_timeout_ms,omitempty"`
	// CustomVersion specifies a specific Unleash version to use (mutually exclusive with ReleaseChannelName)
	CustomVersion string `json:"custom_version,omitempty"`
	// ReleaseChannelName specifies a release channel for automatic version updates (mutually exclusive with CustomVersion)
	ReleaseChannelName string `json:"release_channel_name,omitempty"`
}

// BifrostV1UpdateRequest represents the v1 API request format for updating an unleash instance
type BifrostV1UpdateRequest struct {
	AllowedTeams string `json:"allowed_teams,omitempty"`
	// CustomVersion specifies a specific Unleash version to use (mutually exclusive with ReleaseChannelName)
	CustomVersion string `json:"custom_version,omitempty"`
	// ReleaseChannelName specifies a release channel for automatic version updates (mutually exclusive with CustomVersion)
	ReleaseChannelName string `json:"release_channel_name,omitempty"`
}

// BifrostV1ErrorResponse represents the v1 API error response format
type BifrostV1ErrorResponse struct {
	Error      string            `json:"error"`
	Message    string            `json:"message"`
	Details    map[string]string `json:"details,omitempty"`
	StatusCode int               `json:"status_code"`
}

// UnleashReleaseChannel represents an available release channel from bifrost
type UnleashReleaseChannel struct {
	Name           string     `json:"name"`
	CurrentVersion string     `json:"currentVersion"`
	Type           string     `json:"type"`
	Description    *string    `json:"description,omitempty"`
	LastUpdated    *time.Time `json:"lastUpdated,omitempty"`
}

// BifrostV1ReleaseChannelResponse represents the v1 API response format for a release channel
type BifrostV1ReleaseChannelResponse struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	Type           string `json:"type"`
	Schedule       string `json:"schedule,omitempty"`
	Description    string `json:"description,omitempty"`
	CurrentVersion string `json:"current_version"`
	LastUpdated    string `json:"last_updated,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

func (r *BifrostV1ReleaseChannelResponse) toReleaseChannel() *UnleashReleaseChannel {
	channel := &UnleashReleaseChannel{
		Name:           r.Name,
		CurrentVersion: r.CurrentVersion,
		Type:           r.Type,
	}

	if r.Description != "" {
		channel.Description = &r.Description
	}

	if r.LastUpdated != "" {
		if t, err := time.Parse(time.RFC3339, r.LastUpdated); err == nil {
			channel.LastUpdated = &t
		}
	}

	return channel
}
