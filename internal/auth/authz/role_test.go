package authz_test

import (
	"testing"

	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"k8s.io/utils/ptr"
)

func TestRole_IsGlobal(t *testing.T) {
	targetTeamSlug := slug.Slug("slug")
	tests := map[string]struct {
		role authz.Role
		want bool
	}{
		"global role": {
			role: authz.Role{},
			want: true,
		},
		"role with target team": {
			role: authz.Role{TargetTeamSlug: &targetTeamSlug},
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.role.IsGlobal() != tc.want {
				t.Fatalf("IsGlobal(): expected %t, got %t", tc.want, tc.role.IsGlobal())
			}
		})
	}
}

func TestRole_Targets(t *testing.T) {
	tests := map[string]struct {
		role           authz.Role
		targetTeamSlug slug.Slug
		want           bool
	}{
		"role with target team": {
			role:           authz.Role{TargetTeamSlug: ptr.To(slug.Slug("slug"))},
			targetTeamSlug: slug.Slug("slug"),
			want:           true,
		},
		"role with target team, wrong slug": {
			role:           authz.Role{TargetTeamSlug: ptr.To(slug.Slug("slug"))},
			targetTeamSlug: slug.Slug("wrong"),
			want:           false,
		},
		"role without target team": {
			role:           authz.Role{},
			targetTeamSlug: slug.Slug("slug"),
			want:           false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.role.TargetsTeam(tc.targetTeamSlug) != tc.want {
				t.Fatalf("TargetsTeam(): expected %t, got %t", tc.want, tc.role.TargetsTeam(tc.targetTeamSlug))
			}
		})
	}
}
