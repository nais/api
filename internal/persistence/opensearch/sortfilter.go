package opensearch

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterOpenSearch       = sortfilter.New[*OpenSearch, OpenSearchOrderField, *OpenSearchFilter]()
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

	SortFilterOpenSearch.RegisterFilter(func(ctx context.Context, v *OpenSearch, filter *OpenSearchFilter) bool {
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

		if len(filter.Tiers) > 0 {
			if !slices.Contains(filter.Tiers, v.Tier) {
				return false
			}
		}

		return true
	})

	SortFilterOpenSearchAccess.RegisterSort("ACCESS", func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterOpenSearchAccess.RegisterSort("WORKLOAD", func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
