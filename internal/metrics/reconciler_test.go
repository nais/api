package metrics_test

import (
	"testing"

	"github.com/nais/api/internal/metrics"
	"github.com/nais/api/internal/sqlc"
)

func Test_MeasureReconcilerDurations(t *testing.T) {
	metrics.MeasureReconcilerDuration(sqlc.ReconcilerNameGithubTeam)
	metrics.MeasureReconcilerDuration("")
}
