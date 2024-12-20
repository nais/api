package application

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Application, ApplicationOrderField, *TeamApplicationsFilter](ApplicationOrderFieldName)

func init() {
	SortFilter.RegisterOrderBy(ApplicationOrderFieldName, func(ctx context.Context, a, b *Application) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(ApplicationOrderFieldEnvironment, func(ctx context.Context, a, b *Application) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	})
	SortFilter.RegisterFilter(func(ctx context.Context, v *Application, filter *TeamApplicationsFilter) bool {
		if filter.Name != "" {
			if !strings.Contains(strings.ToLower(v.Name), strings.ToLower(filter.Name)) {
				return false
			}
		}

		if len(filter.Environments) > 0 {
			if !slices.Contains(filter.Environments, v.EnvironmentName) {
				return false
			}
		}

		return true
	})
}
