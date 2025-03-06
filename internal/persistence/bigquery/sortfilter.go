package bigquery

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
	"k8s.io/utils/ptr"
)

var (
	SortFilter       = sortfilter.New[*BigQueryDataset, BigQueryDatasetOrderField, struct{}]()
	SortFilterAccess = sortfilter.New[*BigQueryDatasetAccess, BigQueryDatasetAccessOrderField, struct{}]()
)

type (
	SortFilterTieBreaker = sortfilter.TieBreaker[BigQueryDatasetOrderField]
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, SortFilterTieBreaker{
		Field:     "ENVIRONMENT",
		Direction: ptr.To(model.OrderDirectionAsc),
	})
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, SortFilterTieBreaker{
		Field:     "NAME",
		Direction: ptr.To(model.OrderDirectionAsc),
	})

	SortFilterAccess.RegisterSort("EMAIL", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Email, b.Email)
	})
	SortFilterAccess.RegisterSort("ROLE", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Role, b.Role)
	})
}
