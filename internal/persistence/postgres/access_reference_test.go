package postgres

import (
	"testing"
	"time"

	nais_io_v1 "github.com/nais/pgrator/pkg/api/v1"
)

func TestPostgresAccessReadsBothReferenceShapes(t *testing.T) {
	for _, direct := range []bool{false, true} {
		resource := newPostgresAccessResource(CreatePostgresAccessInput{Postgres: "orders", Branch: "main", TeamSlug: "team", AccessLevel: PostgresAccessLevelRead}, "user@example.com", "existing-access", time.Now().Add(time.Hour))
		if direct {
			spec := resource.Object["spec"].(map[string]any)
			delete(spec, "postgresBranch")
			spec["postgres"] = "orders"
		}
		access, err := toPostgresAccess(resource, "team", "dev")
		if err != nil {
			t.Fatal(err)
		}
		if access.PostgresBranchName != nais_io_v1.PostgresPhysicalName("orders") {
			t.Error("legacy main relationship changed its physical identity")
		}
		if direct && access.PostgresName != "orders" {
			t.Error("direct Postgres reference was lost")
		}
		if access.Username != "user@example.com" || access.AccessLevel != PostgresAccessLevelRead {
			t.Error("access identity or privileges changed")
		}
	}
}
