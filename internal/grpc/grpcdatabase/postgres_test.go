package grpcdatabase

import (
	"testing"

	"github.com/nais/api/internal/persistence/postgres"
	"github.com/nais/api/pkg/apiclient/protoapi"
)

func TestPostgresDatabaseInventory(t *testing.T) {
	got := postgresToProto(&postgres.Postgres{Name: "orders", TeamSlug: "team", EnvironmentName: "dev"})
	if got.Name != "orders" || got.Database != "app" || got.TeamSlug != "team" || got.Environment != "dev" || got.Type != protoapi.DatabaseType_NAIS_POSTGRES {
		t.Errorf("unexpected logical database inventory: %+v", got)
	}
}
