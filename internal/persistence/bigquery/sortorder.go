package bigquery

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter       = sortfilter.New[*BigQueryDataset, BigQueryDatasetOrderField, struct{}](BigQueryDatasetOrderFieldName)
	SortFilterAccess = sortfilter.New[*BigQueryDatasetAccess, BigQueryDatasetAccessOrderField, struct{}](BigQueryDatasetAccessOrderFieldEmail)
)

func init() {
	SortFilter.RegisterOrderBy(BigQueryDatasetOrderFieldName, func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(BigQueryDatasetOrderFieldEnvironment, func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})

	SortFilterAccess.RegisterOrderBy(BigQueryDatasetAccessOrderFieldEmail, func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Email, b.Email)
	})
	SortFilterAccess.RegisterOrderBy(BigQueryDatasetAccessOrderFieldRole, func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Role, b.Role)
	})
}
