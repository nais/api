package application

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Application, ApplicationOrderField, *TeamApplicationsFilter]()

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Application) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Application) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, "NAME")
	SortFilter.RegisterConcurrentSort("STATE", func(ctx context.Context, a *Application) int {
		s, err := GetState(ctx, a)
		if err != nil {
			return int(ApplicationStateUnknown)
		}
		return int(s)
	}, "NAME", "ENVIRONMENT")

	SortFilter.RegisterFilter(matchesFilter)
}
