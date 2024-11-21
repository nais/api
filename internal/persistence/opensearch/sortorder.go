package opensearch

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilterOpenSearch       = sortfilter.New[*OpenSearch, OpenSearchOrderField, struct{}](OpenSearchOrderFieldName)
	SortFilterOpenSearchAccess = sortfilter.New[*OpenSearchAccess, OpenSearchAccessOrderField, struct{}](OpenSearchAccessOrderFieldAccess)
)

func init() {
	SortFilterOpenSearch.RegisterOrderBy(OpenSearchOrderFieldName, func(ctx context.Context, a, b *OpenSearch) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilterOpenSearch.RegisterOrderBy(OpenSearchOrderFieldEnvironment, func(ctx context.Context, a, b *OpenSearch) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterOpenSearchAccess.RegisterOrderBy(OpenSearchAccessOrderFieldAccess, func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.Access, b.Access)
	})
	SortFilterOpenSearchAccess.RegisterOrderBy(OpenSearchAccessOrderFieldWorkload, func(ctx context.Context, a, b *OpenSearchAccess) int {
		return strings.Compare(a.WorkloadReference.Name, b.WorkloadReference.Name)
	})
}
