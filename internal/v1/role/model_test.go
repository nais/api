package role_test

import (
	"testing"

	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/v1/role"
)

func TestRole_IsGlobal(t *testing.T) {
	targetTeamSlug := slug.Slug("slug")
	tests := map[string]struct {
		role role.Role
		want bool
	}{
		"global role": {
			role: role.Role{},
			want: true,
		},
		"team targeted role": {
			role: role.Role{TargetTeamSlug: &targetTeamSlug},
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
	targetTeamSlug := slug.Slug("slug")
	tests := map[string]struct {
		role           role.Role
		targetTeamSlug slug.Slug
		want           bool
	}{
		"role with target team": {
			role:           role.Role{TargetTeamSlug: &targetTeamSlug},
			targetTeamSlug: slug.Slug("slug"),
			want:           true,
		},
		"role with target team, wrong slug": {
			role:           role.Role{TargetTeamSlug: &targetTeamSlug},
			targetTeamSlug: slug.Slug("wrong"),
			want:           false,
		},
		"role without target team": {
			role:           role.Role{},
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
