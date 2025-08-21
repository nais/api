package alerts

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[Alert, AlertOrderField, *TeamAlertsFilter]()

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b Alert) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")

	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b Alert) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, "NAME")

	SortFilter.RegisterFilter(func(ctx context.Context, v Alert, filter *TeamAlertsFilter) bool {
		if len(filter.States) == 0 {
			return true
		}

		if slices.Contains(filter.States, v.GetState()) {
			return true
		}
		return false
	})
}
