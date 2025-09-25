package opensearch

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterOpenSearch       = sortfilter.New[*OpenSearch, OpenSearchOrderField, struct{}]()
	SortFilterOpenSearchAccess = sortfilter.New[*OpenSearchAccess, OpenSearchAccessOrderField, struct{}]()
)

func init() {
	SortFilterOpenSearch.RegisterSort("NAME", func(ctx context.Context, a, b *OpenSearch) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilterOpenSearch.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *OpenSearch) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")
	SortFilterOpenSearch.RegisterConcurrentSort("STATE", func(ctx context.Context, a *OpenSearch) int {
		s, err := State(ctx, a)
		if err != nil {
			return int(OpenSearchStateUnknown)
		}

		return int(s)
	}, "NAME")

	SortFilterOpenSearchAccess.RegisterSort("ACCESS", func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterOpenSearchAccess.RegisterSort("WORKLOAD", func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
