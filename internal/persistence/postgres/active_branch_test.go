package postgres

import (
	"errors"
	"testing"

	"github.com/nais/api/internal/kubernetes/watcher"
)

func TestImplicitActiveBranchNormalizesStaleMetadataWithoutCluster(t *testing.T) {
	for _, observed := range []string{"main", "restored"} {
		t.Run(observed, func(t *testing.T) {
			pg, err := toPostgres(physicalPostgresObject("orders", map[string]any{"activeBranch": observed}), "dev")
			if err != nil {
				t.Fatal(err)
			}
			if pg.ActiveBranch == nil || *pg.ActiveBranch != "main" {
				t.Fatalf("implicit active branch = %v, want main", pg.ActiveBranch)
			}
			view, err := BranchForPostgres(t.Context(), pg, *pg.ActiveBranch)
			if err != nil || view.Name != "main" || view.State != PostgresBranchStateProgressing {
				t.Fatalf("provisioning main = %+v, %v", view, err)
			}
			if _, err := BranchForPostgres(t.Context(), pg, "restored"); !errors.Is(err, &watcher.ErrorNotFound{}) {
				t.Fatalf("explicit retired lookup must not fall back: %v", err)
			}
		})
	}
}
