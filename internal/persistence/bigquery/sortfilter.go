package bigquery

import (
	"context"
	"slices"
	"strings"

	"github.com/nais/api/internal/graph/sortfilter"
)

var (
	SortFilter       = sortfilter.New[*BigQueryDataset, BigQueryDatasetOrderField, *BigQueryDatasetFilter]()
	SortFilterAccess = sortfilter.New[*BigQueryDatasetAccess, BigQueryDatasetAccessOrderField, struct{}]()
)

func init() {
	SortFilter.RegisterSort("NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.Name, b.Name)
	}, "ENVIRONMENT", "_OWNER", "_K8S_RESOURCE_NAME")

	SortFilter.RegisterSort("ENVIRONMENT", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.EnvironmentName, b.EnvironmentName)
	}, "NAME", "_OWNER", "_K8S_RESOURCE_NAME")

	SortFilter.RegisterSort("_OWNER", func(ctx context.Context, a, b *BigQueryDataset) int {
		ownerA := a.TeamSlug.String()
		ownerB := b.TeamSlug.String()

		if a.WorkloadReference != nil {
			ownerA = a.WorkloadReference.Name
		}

		if b.WorkloadReference != nil {
			ownerB = b.WorkloadReference.Name
		}

		return strings.Compare(ownerA, ownerB)
	})

	SortFilter.RegisterSort("_K8S_RESOURCE_NAME", func(ctx context.Context, a, b *BigQueryDataset) int {
		return strings.Compare(a.K8sResourceName, b.K8sResourceName)
	})

	SortFilter.RegisterFilter(func(ctx context.Context, v *BigQueryDataset, filter *BigQueryDatasetFilter) bool {
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

		return true
	})

	SortFilterAccess.RegisterSort("EMAIL", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Email, b.Email)
	})

	SortFilterAccess.RegisterSort("ROLE", func(ctx context.Context, a, b *BigQueryDatasetAccess) int {
		return strings.Compare(a.Role, b.Role)
	})
}
