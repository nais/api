package postgres

import (
	"testing"
	"time"
)

func TestPostgresAccessReference(t *testing.T) {
	resource := newPostgresAccessResource(CreatePostgresAccessInput{Postgres: "orders", TeamSlug: "team", AccessLevel: PostgresAccessLevelRead}, "user@example.com", "existing-access", time.Now().Add(time.Hour))
	access, err := toPostgresAccess(resource, "team", "dev")
	if err != nil {
		t.Fatal(err)
	}
	if access.PostgresName != "orders" || access.Username != "user@example.com" || access.AccessLevel != PostgresAccessLevelRead {
		t.Fatalf("access identity or privileges changed: %+v", access)
	}
	delete(resource.Object["spec"].(map[string]any), "postgres")
	if _, err := toPostgresAccess(resource, "team", "dev"); err == nil {
		t.Fatal("accepted access without a database reference")
	}
}
