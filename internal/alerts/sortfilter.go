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
	}, "STATE", "ENVIRONMENT")
	SortFilter.RegisterSort("STATE", func(ctx context.Context, a, b Alert) int {
		order := map[AlertState]int{
			AlertStateFiring:   0,
			AlertStatePending:  1,
			AlertStateInactive: 2,
		}

		ra := order[a.GetState()]
		rb := order[b.GetState()]

		switch {
		case ra < rb:
			return -1
		case ra > rb:
			return 1
		default:
			return 0
		}
	}, "NAME", "ENVIRONMENT")
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
