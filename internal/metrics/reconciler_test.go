package metrics_test

import (
	"testing"

	"github.com/nais/api/internal/metrics"
)

func Test_MeasureReconcilerDurations(t *testing.T) {
	metrics.MeasureReconcilerDuration("asdf")
	metrics.MeasureReconcilerDuration("")
}
