package unleash

import (
	"context"
	"strings"
	"time"

	"github.com/nais/api/internal/graph/ident"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team"
	"github.com/nais/api/internal/validate"
	"github.com/nais/bifrost/pkg/bifrostclient"
	unleash_nais_io_v1 "github.com/nais/unleasherator/api/v1"
	corev1 "k8s.io/api/core/v1"
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

	releaseChannelName *string // unexported - use ReleaseChannelName() and ReleaseChannel() methods

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

	if u.Spec.ReleaseChannel.Name != "" {
		instance.releaseChannelName = &u.Spec.ReleaseChannel.Name
	}

	return instance
}

func (u UnleashInstance) ID() ident.Ident {
	return newUnleashIdent(u.TeamSlug, u.Name)
}

// ReleaseChannelName returns the name of the release channel (for GraphQL field)
func (u *UnleashInstance) ReleaseChannelName() *string {
	return u.releaseChannelName
}

// ReleaseChannel returns the full release channel object by looking up the channel name from bifrost
func (u *UnleashInstance) ReleaseChannel(ctx context.Context) (*UnleashReleaseChannel, error) {
	if u.releaseChannelName == nil || *u.releaseChannelName == "" {
		return nil, nil
	}

	channels, err := GetReleaseChannels(ctx)
	if err != nil {
		return nil, err
	}

	for _, ch := range channels {
		if ch.Name == *u.releaseChannelName {
			return ch, nil
		}
	}

	return nil, nil
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
	ReleaseChannel *string   `json:"releaseChannel,omitempty"`
}

type CreateUnleashForTeamPayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}

type UpdateUnleashInstanceInput struct {
	TeamSlug       slug.Slug `json:"teamSlug"`
	ReleaseChannel *string   `json:"releaseChannel"`
}

func (i *UpdateUnleashInstanceInput) Validate(_ context.Context) error {
	// No validation needed - all fields are optional except TeamSlug which is required by GraphQL
	return nil
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

// UnleashReleaseChannel represents an available release channel from bifrost
type UnleashReleaseChannel struct {
	Name           string     `json:"name"`
	CurrentVersion string     `json:"currentVersion"`
	Type           string     `json:"type"`
	LastUpdated    *time.Time `json:"lastUpdated,omitempty"`
}

// toReleaseChannel converts the generated bifrostclient.ReleaseChannelResponse to our domain type
func toReleaseChannel(r *bifrostclient.ReleaseChannelResponse) *UnleashReleaseChannel {
	channel := &UnleashReleaseChannel{
		Name:           r.Name,
		CurrentVersion: r.CurrentVersion,
	}

	if r.Type != nil {
		channel.Type = *r.Type
	}

	if r.LastUpdated != nil {
		channel.LastUpdated = r.LastUpdated
	}

	return channel
}

// bifrostUnleashToK8s converts the generated bifrostclient.Unleash to the unleasherator K8s type
func bifrostUnleashToK8s(u *bifrostclient.Unleash) *unleash_nais_io_v1.Unleash {
	if u == nil {
		return nil
	}

	k8s := &unleash_nais_io_v1.Unleash{}

	if u.ApiVersion != nil {
		k8s.APIVersion = *u.ApiVersion
	}
	if u.Kind != nil {
		k8s.Kind = *u.Kind
	}

	if u.Metadata != nil {
		if u.Metadata.Name != nil {
			k8s.Name = *u.Metadata.Name
		}
		if u.Metadata.Namespace != nil {
			k8s.Namespace = *u.Metadata.Namespace
		}
		if u.Metadata.CreationTimestamp != nil {
			k8s.CreationTimestamp.Time = *u.Metadata.CreationTimestamp
		}
	}

	if u.Spec != nil {
		if u.Spec.ReleaseChannel != nil && u.Spec.ReleaseChannel.Name != nil {
			k8s.Spec.ReleaseChannel.Name = *u.Spec.ReleaseChannel.Name
		}
		if u.Spec.CustomImage != nil {
			k8s.Spec.CustomImage = *u.Spec.CustomImage
		}
		if u.Spec.Federation != nil {
			if u.Spec.Federation.Enabled != nil {
				k8s.Spec.Federation.Enabled = *u.Spec.Federation.Enabled
			}
			if u.Spec.Federation.AllowedClusters != nil {
				k8s.Spec.Federation.Clusters = *u.Spec.Federation.AllowedClusters
			}
			if u.Spec.Federation.AllowedTeams != nil {
				k8s.Spec.Federation.Namespaces = *u.Spec.Federation.AllowedTeams
				// Also set ExtraEnvVars for TEAMS_ALLOWED_TEAMS so toUnleashInstance can read it
				allowedTeamsStr := strings.Join(*u.Spec.Federation.AllowedTeams, ",")
				k8s.Spec.ExtraEnvVars = append(k8s.Spec.ExtraEnvVars, corev1.EnvVar{
					Name:  "TEAMS_ALLOWED_TEAMS",
					Value: allowedTeamsStr,
				})
			}
		}
	}

	if u.Status != nil {
		if u.Status.Version != nil {
			k8s.Status.Version = *u.Status.Version
		}
		if u.Status.Connected != nil {
			k8s.Status.Connected = *u.Status.Connected
		}
	}

	return k8s
}
