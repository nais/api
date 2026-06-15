package bucket

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/model"
	"github.com/nais/api/internal/graph/sortfilter"
)

var SortFilter = sortfilter.New[*Bucket, BucketOrderField, *BucketFilter]()

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.GetName(), b.GetName())
	}, "ENVIRONMENT")

	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *Bucket) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME")

	SortFilter.RegisterFilter(func(ctx context.Context, v *Bucket, filter *BucketFilter) bool {
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

		if !model.MatchesLabelFilters(v.Labels, filter.Labels) {
			return false
		}

		return true
	})
}
