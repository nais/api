package metrics_test

import (
	"testing"

	sqlc "github.com/nais/api/internal/database/gensql"
	"github.com/nais/api/internal/metrics"
)

func Test_MeasureReconcilerDurations(t *testing.T) {
	metrics.MeasureReconcilerDuration(sqlc.ReconcilerNameGithubTeam)
	metrics.MeasureReconcilerDuration("")
}
