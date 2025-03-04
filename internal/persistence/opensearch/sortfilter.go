package opensearch

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterOpenSearch       = sortfilter.New[*OpenSearch, OpenSearchOrderField, struct{}]("NAME", model.OrderDirectionAsc)
	SortFilterOpenSearchAccess = sortfilter.New[*OpenSearchAccess, OpenSearchAccessOrderField, struct{}]("ACCESS", model.OrderDirectionAsc)
)

func init() {
	SortFilterOpenSearch.RegisterOrderBy("NAME", func(ctx context.Context, a, b *OpenSearch) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterOpenSearch.RegisterOrderBy("ENVIRONMENT", func(ctx context.Context, a, b *OpenSearch) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterOpenSearchAccess.RegisterOrderBy("ACCESS", func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterOpenSearchAccess.RegisterOrderBy("WORKLOAD", func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
