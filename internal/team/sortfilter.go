package team

import (
	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Team, TeamOrderField, struct{}]()

// func init() {
// 	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Application) int {
// 		return strings.Compare(a.GetName(), b.GetName())
// 	}, "ENVIRONMENT")
// 	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Application) int {
// 		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
// 	}, "NAME")
// }
