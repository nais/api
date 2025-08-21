package alerts

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[Alert, AlertOrderField, struct{}]()

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b Alert) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")

	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b Alert) int {
		return strings.Compare(a.GetEnvironmentName(), b.GetEnvironmentName())
	}, "NAME")
}
