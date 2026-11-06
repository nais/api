package teamscope_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/auth/authz"
	"github.com/nais/api/internal/slug"
	"github.com/nais/api/internal/team/teamscope"
)

type scopeUser struct {
	authz.AuthenticatedUser
	serviceAccount bool
}

func (u scopeUser) IsServiceAccount() bool { return u.serviceAccount }

func TestResolve(t *testing.T) {
	roles := []*authz.Role{{Name: "Admin"}}
	memberships := make([]slug.Slug, 25)
	for i := range memberships {
		memberships[i] = slug.Slug(fmt.Sprintf("team-%02d", i))
		roles = append(roles, &authz.Role{Name: "Team member", TargetTeamSlug: new(memberships[i])})
	}
	roles = append(roles, roles[1])
	ctx := authz.ContextWithActor(t.Context(), scopeUser{}, roles)
	serviceCtx := authz.ContextWithActor(t.Context(), scopeUser{serviceAccount: true}, roles)
	adminCtx := authz.ContextWithActor(t.Context(), scopeUser{}, []*authz.Role{{Name: "Admin"}})

	for _, tt := range []struct {
		name  string
		ctx   context.Context
		scope *teamscope.TeamScopeInput
		want  []slug.Slug
	}{
		{"omitted", ctx, nil, nil},
		{"empty input", ctx, &teamscope.TeamScopeInput{}, nil},
		{"explicit empty", ctx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{}}, []slug.Slug{}},
		{"explicit teams", ctx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other"}}, []slug.Slug{"other"}},
		{"duplicate explicit teams", ctx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other", "team-24", "other"}}, []slug.Slug{"other", "team-24"}},
		{"all memberships", ctx, &teamscope.TeamScopeInput{MemberOfViewer: new(true)}, memberships},
		{"union", ctx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"team-24", "other", "other"}, MemberOfViewer: new(true)}, append([]slug.Slug{"team-24", "other"}, memberships[:24]...)},
		{"empty explicit teams with memberships", ctx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{}, MemberOfViewer: new(true)}, memberships},
		{"membership false", ctx, &teamscope.TeamScopeInput{MemberOfViewer: new(false)}, nil},
		{"membership false with explicit teams", ctx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other"}, MemberOfViewer: new(false)}, []slug.Slug{"other"}},
		{"admin without memberships", adminCtx, &teamscope.TeamScopeInput{MemberOfViewer: new(true)}, []slug.Slug{}},
		{"admin with explicit teams", adminCtx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other"}, MemberOfViewer: new(true)}, []slug.Slug{"other"}},
		{"service account", serviceCtx, &teamscope.TeamScopeInput{MemberOfViewer: new(true)}, []slug.Slug{}},
		{"service account explicit teams", serviceCtx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other"}}, []slug.Slug{"other"}},
		{"service account explicit teams with memberships", serviceCtx, &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other"}, MemberOfViewer: new(true)}, []slug.Slug{"other"}},
		{"no actor", t.Context(), &teamscope.TeamScopeInput{MemberOfViewer: new(true)}, []slug.Slug{}},
		{"no actor with explicit teams", t.Context(), &teamscope.TeamScopeInput{TeamSlugs: []slug.Slug{"other"}, MemberOfViewer: new(true)}, []slug.Slug{"other"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if diff := cmp.Diff(tt.want, tt.scope.Resolve(tt.ctx)); diff != "" {
				t.Errorf("team selection (-want +got):\n%s", diff)
			}
		})
	}
}
