package bucket

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Bucket, BucketOrderField, struct{}]("NAME", model.OrderDirectionAsc)

func init() {
	SortFilter.RegisterOrderBy("NAME", func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy("ENVIRONMENT", func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})
}
