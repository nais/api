package bucket

import (
	"context"
	"strings"

	"github.com/nais/api/internal/v1/graphv1/sortfilter"
)

var SortFilter = sortfilter.New[*Bucket, BucketOrderField, struct{}](BucketOrderFieldName)

func init() {
	SortFilter.RegisterOrderBy(BucketOrderFieldName, func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.GetName(), b.GetName())
	})
	SortFilter.RegisterOrderBy(BucketOrderFieldEnvironment, func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	})
}
