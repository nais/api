package unleash

import (
	"context"
	"strings"

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

	return &UnleashInstance{
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
	TeamSlug slug.Slug `json:"team"`
}

type CreateUnleashForTeamPayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}

type RevokeTeamAccessToUnleashInput struct {
	TeamSlug        slug.Slug `json:"team"`
	RevokedTeamSlug slug.Slug `json:"revokedTeam"`
}

type RevokeTeamAccessToUnleashPayload struct {
	Unleash *UnleashInstance `json:"unleash,omitempty"`
}
