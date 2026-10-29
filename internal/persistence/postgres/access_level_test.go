package postgres

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/api/internal/slug"
	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
)

func TestPostgresAccessLevels(t *testing.T) {
	for _, tt := range []struct {
		level PostgresAccessLevel
		crd   string
	}{
		{level: PostgresAccessLevelRead, crd: "read"},
		{level: PostgresAccessLevelReadWrite, crd: "readwrite"},
		{level: PostgresAccessLevelReadWriteCreate, crd: "readwritecreate"},
		{level: PostgresAccessLevelMaintain, crd: "maintain"},
	} {
		t.Run(tt.level.String(), func(t *testing.T) {
			var level PostgresAccessLevel
			if err := level.UnmarshalGQL(tt.level.String()); err != nil {
				t.Fatalf("UnmarshalGQL(): %v", err)
			}
			if level != tt.level || !level.IsValid() {
				t.Fatalf("level = %q, want valid %q", level, tt.level)
			}
			var output bytes.Buffer
			level.MarshalGQL(&output)
			if got, want := output.String(), `"`+tt.level.String()+`"`; got != want {
				t.Errorf("MarshalGQL() = %s, want %s", got, want)
			}
			expiresAt := time.Now().Add(time.Hour).Truncate(time.Second)
			resource := newPostgresAccessResource(CreatePostgresAccessInput{
				Postgres: "orders", Branch: "main", TeamSlug: slug.Slug("team-a"), AccessLevel: level,
			}, "user@example.com", "personal-access", expiresAt)
			wantSpec := map[string]any{
				"postgresBranch": nais_io_v1.PostgresBranchObjectName("orders", "main"),
				"username":       "user@example.com", "accessLevel": tt.crd, "expiresAt": expiresAt.Format(time.RFC3339),
			}
			if diff := cmp.Diff(wantSpec, resource.Object["spec"]); diff != "" {
				t.Errorf("resource spec mismatch (-want +got):\n%s", diff)
			}
			access, err := toPostgresAccess(resource, slug.Slug("team-a"), "dev")
			if err != nil {
				t.Fatalf("toPostgresAccess(): %v", err)
			}
			if access.AccessLevel != level || access.Username != "user@example.com" || !access.ExpiresAt.Equal(expiresAt) {
				t.Errorf("unexpected access after round trip: %+v", access)
			}
		})
	}
}

func TestPostgresAccessLevelRejectsInvalidInput(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
	}{
		{name: "empty", value: ""},
		{name: "lowercase", value: "maintain"},
		{name: "old name", value: "DBA"},
		{name: "admin", value: "ADMIN"},
		{name: "unknown", value: "UNKNOWN"},
		{name: "non-string", value: 42},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var level PostgresAccessLevel
			if err := level.UnmarshalGQL(tt.value); err == nil {
				t.Fatalf("UnmarshalGQL(%v) must fail", tt.value)
			}
		})
	}
}
