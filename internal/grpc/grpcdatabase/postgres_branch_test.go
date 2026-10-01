package grpcdatabase

import (
	"testing"

	"github.com/nais/api/internal/persistence/postgres"
)

func TestPostgresBranchDatabaseName(t *testing.T) {
	for _, tt := range []struct {
		branch string
		want   string
	}{
		{branch: "main", want: "orders"},
		{branch: "recovered", want: "orders/recovered"},
	} {
		t.Run(tt.branch, func(t *testing.T) {
			got := postgresBranchToProto(&postgres.PostgresBranch{PostgresName: "orders", Name: tt.branch})
			if got.Name != tt.want {
				t.Errorf("database name = %q, want %q", got.Name, tt.want)
			}
		})
	}
}
