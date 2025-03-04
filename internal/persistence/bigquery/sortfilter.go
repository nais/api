package bigquery

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter       = sortfilter.New[*BigQueryDataset, BigQueryDatasetOrderField, struct{}]("NAME", model.OrderDirectionAsc)
	SortFilterAccess = sortfilter.New[*BigQueryDatasetAccess, BigQueryDatasetAccessOrderField, struct{}]("EMAIL", model.OrderDirectionAsc)
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterAccess.RegisterSort("EMAIL", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Email, b.Email)
	})
	SortFilterAccess.RegisterSort("ROLE", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Role, b.Role)
	})
}
