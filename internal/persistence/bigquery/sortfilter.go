package bigquery

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter       = sortfilter.New[*BigQueryDataset, BigQueryDatasetOrderField, struct{}]()
	SortFilterAccess = sortfilter.New[*BigQueryDatasetAccess, BigQueryDatasetAccessOrderField, struct{}]()
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilterAccess.RegisterSort("EMAIL", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Email, b.Email)
	})
	SortFilterAccess.RegisterSort("ROLE", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Role, b.Role)
	})
}
