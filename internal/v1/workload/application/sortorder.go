package application

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
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
		return strings.Contains(strings.ToLower(v.Name), strings.ToLower(filter.Name))
	})
}
