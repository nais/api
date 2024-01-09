package database_test

import (
	"testing"

	"github.com/nais/api/internal/slug"

	db "github.com/nais/api/internal/database"
	"github.com/stretchr/testify/assert"
)

func TestRole_IsGlobal(t *testing.T) {
	targetTeamSlug := slug.Slug("slug")
	r1 := db.Role{TargetTeamSlug: &targetTeamSlug}
	r2 := db.Role{}
	assert.False(t, r1.IsGlobal())
	assert.True(t, r2.IsGlobal())
}

func TestRole_Targets(t *testing.T) {
	slug1 := slug.Slug("slug1")
	slug2 := slug.Slug("slug2")
	r1 := db.Role{TargetTeamSlug: &slug1}
	r2 := db.Role{TargetTeamSlug: &slug2}
	r3 := db.Role{}

	assert.True(t, r1.TargetsTeam(slug1))
	assert.False(t, r2.TargetsTeam(slug1))
	assert.False(t, r3.TargetsTeam(slug1))

	assert.False(t, r1.TargetsTeam(slug2))
	assert.True(t, r2.TargetsTeam(slug2))
	assert.False(t, r3.TargetsTeam(slug2))
}
