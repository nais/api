package bucket

import (
	"context"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Bucket, BucketOrderField, struct{}]("NAME")

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})
}
